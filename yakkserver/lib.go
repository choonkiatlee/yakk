package yakkserver

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sethvargo/go-diceware/diceware"
)

func Hello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{\"Hello\":\"World\"}"))
}

const (
	YAKKMSG_CREATEROOM  = "YAKKMSG_CREATEROOM"
	YAKKMSG_JOINROOM    = "YAKKMSG_JOINROOM"
	YAKKMSG_ROOMCREATED = "YAKKMSG_ROOMCREATED"
	YAKKMSG_ROOMJOINED  = "YAKKMSG_ROOMJOINED"

	YAKKMSG_REQUESTOFFER = "YAKKMSG_REQUESTOFFER"
	YAKKMSG_PAKEEXCHANGE = "YAKKMSG_PAKEEXCHANGE"

	YAKKMSG_OFFER             = "YAKKMSG_OFFER"
	YAKKMSG_ANSWER            = "YAKKMSG_ANSWER"
	YAKKMSG_NEW_ICE_CANDIDATE = "YAKKMSG_NEW_ICE_CANDIDATE"
	YAKKMSG_CONNECTED         = "YAKKMSG_CONNECTED"

	YAKKMSG_ERROR        = "YAKKMSG_ERROR"
	YAKKMSG_DISCONNECTED = "YAKKMSG_DISCONNECTED"
)

type YakkMailboxMessage struct {
	Msg_type  string
	Payload   string
	Sender    int
	Recipient int
}

type YakkMailBox struct {
	Name             string
	lastAccessedTime time.Time
	ID               int
	Conn             *websocket.Conn
	ClientConn       *websocket.Conn
	ServerConn       *websocket.Conn
}

type YakkMailRoom struct {
	Name          string
	YakkMailBoxes map[int]*YakkMailBox // Map of client ids, each keyed to one mailbox
	OwnerID       int
	mailBoxMux    sync.Mutex
}

var upgrader = websocket.Upgrader{} // use default options

var yakkMailBoxes = make(map[string]*YakkMailBox)

var yakkMailRooms = make(map[string]*YakkMailRoom)

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

func CreateMailRoom(server_ws_conn *websocket.Conn) YakkMailboxMessage {
	randomWordsList, err := diceware.Generate(1)
	if err != nil {
		fmt.Println(err)
	}

	yakkMailBoxesMap := make(map[int]*YakkMailBox)
	yakkMailBoxesMap[0] = &YakkMailBox{
		ID:               0,
		Conn:             server_ws_conn,
		lastAccessedTime: time.Now(),
	}

	// OwnerID is always 0
	yakkMailRoom := YakkMailRoom{
		Name:          strings.Join(randomWordsList, "_"),
		OwnerID:       0,
		YakkMailBoxes: yakkMailBoxesMap,
		mailBoxMux:    sync.Mutex{},
	}

	yakkMailRooms[yakkMailRoom.Name] = &yakkMailRoom

	mailboxReplyMsg := YakkMailboxMessage{
		Msg_type: YAKKMSG_ROOMCREATED,
		Payload:  yakkMailRoom.Name,
	}
	return mailboxReplyMsg
}

func JoinMailBox(roomID string, client_ws_conn *websocket.Conn) YakkMailboxMessage {

	var ReplyMsg YakkMailboxMessage
	if yakkMailBox, ok := yakkMailBoxes[roomID]; ok {
		// join a room instead
		yakkMailBox.ClientConn = client_ws_conn
		FusePipes2(yakkMailBox)
		fmt.Println(fmt.Sprintf("All Joined Room: %s", roomID))
		ReplyMsg = YakkMailboxMessage{
			Msg_type: YAKKMSG_ROOMJOINED,
			Payload:  roomID,
		}
	} else {
		fmt.Println(fmt.Sprintf("No such room: %s.", roomID))
		ReplyMsg = YakkMailboxMessage{
			Msg_type: YAKKMSG_ERROR,
			Payload:  "No such room",
		}
		client_ws_conn.WriteJSON(ReplyMsg)
		var extraMsg YakkMailboxMessage
		err := ReadOneWS(client_ws_conn, &extraMsg)
		if err != nil {
			log.Println("Could not read.")
			panic(err)
		}
		return JoinMailBox(extraMsg.Payload, client_ws_conn)
	}
	return ReplyMsg
}

func FusePipes2(yakkMailBox *YakkMailBox) {
	server_conn := yakkMailBox.ServerConn
	client_conn := yakkMailBox.ClientConn

	go ConnectPipeOneDirectional(server_conn, client_conn)
	go ConnectPipeOneDirectional(client_conn, server_conn)
}

func FusePipes(yakkMailBox *YakkMailBox) {
	server_conn := yakkMailBox.ServerConn
	client_conn := yakkMailBox.ClientConn

	messageType, serverReader, err := server_conn.NextReader()
	if err != nil {
		fmt.Println(err)
	}

	serverWriter, err := server_conn.NextWriter(messageType)
	if err != nil {
		fmt.Println(err)
	}

	messageType, clientReader, err := client_conn.NextReader()
	if err != nil {
		fmt.Println(err)
	}

	clientWriter, err := client_conn.NextWriter(messageType)
	if err != nil {
		fmt.Println(err)
	}

	brokerError := make(chan struct{}, 1)

	go broker2(serverWriter, serverReader, brokerError)
	go broker2(clientWriter, clientReader, brokerError)

	<-brokerError
	client_conn.Close()
	fmt.Println("Closing client...")

	// Block until pipe Error is triggered. We only close the client, and keep the
	// server alive
	// <-pipeError
	// client_conn.Close()
	// fmt.Println("Closing Connections....")
}

func ConnectPipeOneDirectional(conn1 *websocket.Conn, conn2 *websocket.Conn) {
	// All messages from conn1 written to conn2
	// Todo: how to keep the server alive when the client dies?
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
	// pipeError <- struct{}{}
	conn1.Close()
	conn2.Close()
	fmt.Println("Closing Connections....")
}

func HandleMessage(w http.ResponseWriter, req *http.Request) {

	roomID := strings.TrimPrefix(req.URL.Path, "/ws/")
	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}

	var ReplyMsg YakkMailboxMessage
	if len(roomID) > 0 {
		fmt.Println(roomID)
		ReplyMsg = JoinMailBox(roomID, ws)
	} else {
		ReplyMsg = CreateMailbox(ws)
	}
	// hold the websockets and return.
	ws.WriteJSON(ReplyMsg)
}

func CleanMailBoxes() {
	// Remove all YakkMailBoxes older than 10min
	for {
		currentTime := time.Now()
		for mailBoxName, yakkMailBox := range yakkMailBoxes {
			if currentTime.Sub(yakkMailBox.lastAccessedTime) > 10*time.Minute {
				delete(yakkMailBoxes, mailBoxName)
				fmt.Println("Deleted Mailbox ", mailBoxName)
			}
		}
		time.Sleep(2 * time.Minute)
	}
}

func ReadOneWS(ws *websocket.Conn, output interface{}) error {
	_, r, err := ws.NextReader()
	if err != nil {
		return err
	}
	msg, err := ioutil.ReadAll(r)
	return json.Unmarshal(msg, output)
}

// This does the actual data transfer.
// The broker only closes the Read side.
func broker2(dst io.Writer, src io.Reader, srcClosed chan struct{}) {
	// We can handle errors in a finer-grained manner by inlining io.Copy (it's
	// simple, and we drop the ReaderFrom or WriterTo checks for
	// net.Conn->net.Conn transfers, which aren't needed). This would also let
	// us adjust buffersize.
	// Create a buffer of 4KB. This is to prevent readwrite errors as in:
	// https://github.com/pion/datachannel/issues/59
	buf := make([]byte, 4<<10)
	_, err := io.CopyBuffer(dst, src, buf)

	if err != nil {
		// We should be able to fairly simply ignore this
		log.Printf("Copy error: %s\n", err)
		srcClosed <- struct{}{}
	}
}

// Todo: Fact Check this
func StapleConnections2(conn1 io.ReadWriteCloser, conn2 io.ReadWriteCloser, keepConn2Alive bool) {
	// channels to wait on the close event for each connection
	brokerError := make(chan struct{}, 1)

	go broker2(conn1, conn2, brokerError)
	go broker2(conn2, conn1, brokerError)

	<-brokerError
	conn1.Close()
	fmt.Println("Closing conn1...")

}
