package yakkserver

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sethvargo/go-diceware/diceware"
)

func Hello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{\"Hello\":\"World\"}"))
}

const (
	YAKKMSG_CREATEROOM        = "YAKKMSG_CREATEROOM"
	YAKKMSG_ROOMCREATED       = "YAKKMSG_ROOMCREATED"
	YAKKMSG_PAKEEXCHANGE      = "YAKKMSG_PAKEEXCHANGE"
	YAKKMSG_REQUESTOFFER      = "YAKKMSG_REQUESTOFFER"
	YAKKMSG_OFFER             = "YAKKMSG_OFFER"
	YAKKMSG_JOINROOM          = "YAKKMSG_JOINROOM"
	YAKKMSG_NEW_ICE_CANDIDATE = "YAKKMSG_NEW_ICE_CANDIDATE"
	YAKKMSG_CONNECTED         = "YAKKMSG_CONNECTED"
	YAKKMSG_ANSWER            = "YAKKMSG_ANSWER"
)

type YakkMailboxMessage struct {
	Msg_type string
	Payload  string
}

type YakkMailBox struct {
	Name                string
	OwnerConnectionInfo string
	lastAccessedTime    time.Time
	ClientConn          *websocket.Conn
	ServerConn          *websocket.Conn
}

var upgrader = websocket.Upgrader{} // use default options

var yakkMailBoxes = make(map[string]*YakkMailBox)

func CreateMailbox(server_ws_conn *websocket.Conn) YakkMailboxMessage {

	// to do: err out if room alr exists
	// create a new YakkMailRoom
	randomWordsList, err := diceware.Generate(2)
	if err != nil {
		fmt.Println(err)
	}
	yakkMailBox := YakkMailBox{
		Name:             strings.Join(randomWordsList, "_"),
		ServerConn:       server_ws_conn,
		lastAccessedTime: time.Now(),
	}

	yakkMailBoxes[yakkMailBox.Name] = &yakkMailBox
	mailboxReplyMsg := YakkMailboxMessage{
		Msg_type: YAKKMSG_ROOMCREATED,
		Payload:  yakkMailBox.Name,
	}
	return mailboxReplyMsg
}

func FusePipes(yakkMailBox *YakkMailBox) {
	server_conn := yakkMailBox.ServerConn
	client_conn := yakkMailBox.ClientConn

	go ConnectPipeOneDirectional(server_conn, client_conn)
	go ConnectPipeOneDirectional(client_conn, server_conn)
}

func ConnectPipeOneDirectional(conn1 *websocket.Conn, conn2 *websocket.Conn) {
	// All messages from conn1 written to conn2
	for {
		messageType, r, err := conn1.NextReader()
		if err != nil {
			fmt.Println(err)
			break
		}
		w, err := conn2.NextWriter(messageType)
		if err != nil {
			fmt.Println(err)
			break
		}
		if _, err := io.Copy(w, r); err != nil {
			fmt.Println(err)
			break
		}
		if err := w.Close(); err != nil {
			break
		}
	}
	// if we get any errors, we assume that the clients have connected and closed their
	// connections, so we close and do the same too.
	conn1.Close()
	conn2.Close()
	fmt.Println("Closing Connections....")
}

func HandleJoinRoom(yakkMailBox *YakkMailBox, client_ws_conn *websocket.Conn) {
	yakkMailBox.ClientConn = client_ws_conn
	// Fuse the pipes upon joining the room
	FusePipes(yakkMailBox)
}

func HandleMessage(w http.ResponseWriter, req *http.Request) {

	roomID := strings.TrimPrefix(req.URL.Path, "/ws/")
	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}
	if yakkMailBox, ok := yakkMailBoxes[roomID]; ok {
		// join a room instead
		HandleJoinRoom(yakkMailBox, ws)
		fmt.Println(fmt.Sprintf("All Joined Room: %s", roomID))
	} else {
		roomCreatedReplyMsg := CreateMailbox(ws)
		ws.WriteJSON(roomCreatedReplyMsg)
	}
	// hold the websockets and return.
}

func CleanMailBoxes() {
	// Remove all YakkMailBoxes older than 10min
	for {
		currentTime := time.Now()
		for mailBoxName, yakkMailBox := range yakkMailBoxes {
			if currentTime.Sub(yakkMailBox.lastAccessedTime) > 10*time.Minute {
				delete(yakkMailBoxes, mailBoxName)
				fmt.Print("Deleted Mailbox ", mailBoxName)
			}
		}
		time.Sleep(2 * time.Minute)
	}
}
