package yakkserver

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
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
	YAKKMSG_NEWJOINER   = "YAKKMSG_NEWJOINER"

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
	lastAccessedTime time.Time
	ID               int
	Conn             *websocket.Conn
}

type YakkMailRoom struct {
	Name               string
	YakkMailBoxes      map[int]*YakkMailBox // Map of client ids, each keyed to one mailbox
	OwnerID            int
	messagesQueue      chan []byte
	keepAliveInMinutes int
}

var upgrader = websocket.Upgrader{} // use default options

// var yakkMailBoxes = make(map[string]*YakkMailBox)
var yakkMailRooms = make(map[string]*YakkMailRoom)

func CreateMailRoom(server_ws_conn *websocket.Conn, keepAlive bool) YakkMailboxMessage {
	randomWordsList, err := diceware.Generate(1)
	if err != nil {
		log.Error().Msg(err.Error())
	}

	// OwnerID is always 0
	var keepAliveInMinutes int
	if keepAlive {
		keepAliveInMinutes = 10
	} else {
		keepAliveInMinutes = 1
	}
	yakkMailRoom := YakkMailRoom{
		Name:               strings.Join(randomWordsList, "_"),
		OwnerID:            0,
		YakkMailBoxes:      make(map[int]*YakkMailBox),
		messagesQueue:      make(chan []byte),
		keepAliveInMinutes: keepAliveInMinutes,
	}

	yakkMailRooms[yakkMailRoom.Name] = &yakkMailRoom
	id := CreateMailBox(yakkMailRoom.Name, server_ws_conn)
	yakkMailRoom.OwnerID = id

	mailboxReplyMsg := YakkMailboxMessage{
		Msg_type:  YAKKMSG_ROOMCREATED,
		Payload:   yakkMailRoom.Name,
		Sender:    -1,
		Recipient: id,
	}
	go OperateMailRoomWrites(&yakkMailRoom)
	return mailboxReplyMsg
}

func CreateMailBox(roomID string, conn *websocket.Conn) int {

	// Retrieve the mailroom
	yakkMailRoom := yakkMailRooms[roomID]

	// Get the next available connection name
	// super simple version
	id := len(yakkMailRoom.YakkMailBoxes)

	yakkMailBox := YakkMailBox{
		ID:               id,
		Conn:             conn,
		lastAccessedTime: time.Now(),
	}
	yakkMailRoom.YakkMailBoxes[id] = &yakkMailBox
	go OperateMailBoxReads(id, yakkMailRoom)
	return id
}

func JoinMailRoom(roomID string, client_ws_conn *websocket.Conn) YakkMailboxMessage {

	var ReplyMsg YakkMailboxMessage
	if yakkMailRoom, ok := yakkMailRooms[roomID]; ok {
		// Room available. Join Room.
		id := CreateMailBox(roomID, client_ws_conn)
		ReplyMsg = YakkMailboxMessage{
			Msg_type:  YAKKMSG_ROOMJOINED,
			Payload:   fmt.Sprint(id),
			Sender:    yakkMailRoom.OwnerID,
			Recipient: id,
		}
	} else {
		log.Info().Msg(fmt.Sprintf("No such room: %s.", roomID))
		ReplyMsg = YakkMailboxMessage{
			Msg_type: YAKKMSG_ERROR,
			Payload:  "No such room",
		}
		client_ws_conn.WriteJSON(ReplyMsg)
		var extraMsg YakkMailboxMessage
		err := ReadOneWS(client_ws_conn, &extraMsg)
		if err != nil {
			log.Info().Msg("Could not read.")
			panic(err)
		}
		return JoinMailRoom(extraMsg.Payload, client_ws_conn)
	}
	return ReplyMsg
}

func HandleMessage(w http.ResponseWriter, req *http.Request) {

	var roomID string = ""
	var keepAlive bool = false
	var err error
	roomIDs, ok := req.URL.Query()["roomID"]
	if ok || len(roomIDs) >= 1 {
		roomID = roomIDs[0]
	}

	keepAlives, ok := req.URL.Query()["keepAlive"]
	if ok || len(keepAlives) >= 1 {
		keepAlive, err = strconv.ParseBool(keepAlives[0])
		if err != nil {
			log.Info().Msg("Keep Conn Alive parameter not a bool-like value")
			return
		}
	}

	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Print("Problem upgrading the ws conn with error: ", err)
		return
	}

	var ReplyMsg YakkMailboxMessage
	if len(roomID) > 0 {
		log.Debug().Msg(roomID)
		ReplyMsg = JoinMailRoom(roomID, ws)
	} else {
		ReplyMsg = CreateMailRoom(ws, keepAlive)
	}
	// hold the websockets and return.
	ws.WriteJSON(ReplyMsg)
}

func OperateMailRoomWrites(yakkMailRoom *YakkMailRoom) {
	var mailBoxMsg YakkMailboxMessage
	for message := range yakkMailRoom.messagesQueue {
		err := json.Unmarshal(message, &mailBoxMsg)
		if err != nil {
			log.Error().Msg(err.Error())
		}
		// Forward it on to the recipient
		if mailBox, ok := yakkMailRoom.YakkMailBoxes[mailBoxMsg.Recipient]; ok {
			mailBox.Conn.WriteMessage(websocket.BinaryMessage, message)
		}
	}
}

func OperateMailBoxReads(ID int, yakkMailRoom *YakkMailRoom) {
	conn := yakkMailRoom.YakkMailBoxes[ID].Conn
	for {
		_, m, err := conn.ReadMessage()
		if err != nil {
			log.Error().Msgf("Could not read: %s", err.Error())
			// If you are the last open connection, close the channel?
			conn.Close()
			delete(yakkMailRoom.YakkMailBoxes, ID)
			return
		}
		yakkMailRoom.messagesQueue <- m
	}
}

// func CleanMailBoxes() {
// 	// Remove all YakkMailBoxes older than 10min
// 	for {
// 		currentTime := time.Now()
// 		for mailBoxName, yakkMailBox := range yakkMailBoxes {
// 			if currentTime.Sub(yakkMailBox.lastAccessedTime) > 10*time.Minute {
// 				delete(yakkMailBoxes, mailBoxName)
// 				fmt.Println("Deleted Mailbox ", mailBoxName)
// 			}
// 		}
// 		time.Sleep(2 * time.Minute)
// 	}
// }

func CleanMailRooms() {
	// Remove all YakkMailRooms with no valid YakkMailBoxes unless the keepalive parameter is set
	sleepTime := 2
	for {
		for mailRoomName, mailRoom := range yakkMailRooms {
			log.Print("Trying to clean up ", mailRoomName, " with ", len(mailRoom.YakkMailBoxes), " mailboxes")
			if len(mailRoom.YakkMailBoxes) == 0 {
				if mailRoom.keepAliveInMinutes <= 0 {
					close(mailRoom.messagesQueue) // safe to close this because zero mailboxes: i.e. no more writing to the message queue
					delete(yakkMailRooms, mailRoomName)
					log.Print("Deleted Mailroom with no mailboxes: ", mailRoomName)
				} else {
					mailRoom.keepAliveInMinutes -= sleepTime
				}
			}
			time.Sleep(time.Duration(sleepTime) * time.Second)
		}
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
	log.Debug().Msg("Closing conn1...")

}

// func FusePipes2(yakkMailBox *YakkMailBox) {
// 	server_conn := yakkMailBox.ServerConn
// 	client_conn := yakkMailBox.ClientConn

// 	go ConnectPipeOneDirectional(server_conn, client_conn)
// 	go ConnectPipeOneDirectional(client_conn, server_conn)
// }

// func FusePipes(yakkMailBox *YakkMailBox) {
// 	server_conn := yakkMailBox.ServerConn
// 	client_conn := yakkMailBox.ClientConn

// 	messageType, serverReader, err := server_conn.NextReader()
// 	if err != nil {
// 		fmt.Println(err)
// 	}

// 	serverWriter, err := server_conn.NextWriter(messageType)
// 	if err != nil {
// 		fmt.Println(err)
// 	}

// 	messageType, clientReader, err := client_conn.NextReader()
// 	if err != nil {
// 		fmt.Println(err)
// 	}

// 	clientWriter, err := client_conn.NextWriter(messageType)
// 	if err != nil {
// 		fmt.Println(err)
// 	}

// 	brokerError := make(chan struct{}, 1)

// 	go broker2(serverWriter, serverReader, brokerError)
// 	go broker2(clientWriter, clientReader, brokerError)

// 	<-brokerError
// 	client_conn.Close()
// 	fmt.Println("Closing client...")

// 	// Block until pipe Error is triggered. We only close the client, and keep the
// 	// server alive
// 	// <-pipeError
// 	// client_conn.Close()
// 	// fmt.Println("Closing Connections....")
// }

// func ConnectPipeOneDirectional(conn1 *websocket.Conn, conn2 *websocket.Conn) {
// 	// All messages from conn1 written to conn2
// 	// Todo: how to keep the server alive when the client dies?
// 	for {
// 		messageType, r, err := conn1.NextReader()
// 		if err != nil {
// 			fmt.Println(err)
// 			break
// 		}
// 		w, err := conn2.NextWriter(messageType)
// 		if err != nil {
// 			fmt.Println(err)
// 			break
// 		}
// 		if _, err := io.Copy(w, r); err != nil {
// 			fmt.Println(err)
// 			break
// 		}
// 		if err := w.Close(); err != nil {
// 			break
// 		}
// 	}
// 	// if we get any errors, we assume that the clients have connected and closed their
// 	// connections, so we close and do the same too.
// 	// pipeError <- struct{}{}
// 	conn1.Close()
// 	conn2.Close()
// 	fmt.Println("Closing Connections....")
// }

// func CreateMailbox(server_ws_conn *websocket.Conn) YakkMailboxMessage {

// 	// to do: err out if room alr exists
// 	// create a new YakkMailRoom
// 	randomWordsList, err := diceware.Generate(2)
// 	if err != nil {
// 		fmt.Println(err)
// 	}
// 	yakkMailBox := YakkMailBox{
// 		Name:             strings.Join(randomWordsList, "_"),
// 		ServerConn:       server_ws_conn,
// 		lastAccessedTime: time.Now(),
// 	}

// 	yakkMailBoxes[yakkMailBox.Name] = &yakkMailBox
// 	mailboxReplyMsg := YakkMailboxMessage{
// 		Msg_type: YAKKMSG_ROOMCREATED,
// 		Payload:  yakkMailBox.Name,
// 	}
// 	return mailboxReplyMsg
// }
