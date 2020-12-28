package yakk

import (
	"github.com/choonkiatlee/yakk/yakkserver"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

type YakkMailBoxConnection struct {
	Name         string
	SenderID     int // SenderID from the current object's perspective.
	RecipientID  int
	State        string
	SessionKey   []byte
	WriteChannel chan yakkserver.YakkMailboxMessage
	RecvChannel  chan yakkserver.YakkMailboxMessage
}

func (yakkmailBox *YakkMailBoxConnection) SendMsg(msg_type string, payload string) {
	msg := yakkserver.YakkMailboxMessage{
		Msg_type:  msg_type,
		Payload:   payload,
		Sender:    yakkmailBox.SenderID,
		Recipient: yakkmailBox.RecipientID,
	}
	yakkmailBox.WriteChannel <- msg
}

func (yakkMailBox *YakkMailBoxConnection) Close() {
	// Close the recv channel.
	close(yakkMailBox.RecvChannel)
}

type YakkMailRoomConnection struct {
	Name         string
	OwnerID      int
	WriteChannel chan yakkserver.YakkMailboxMessage // All mailboxes hold a reference to this writechannel for convenience. One writechannel per mailroom.
	MailBoxes    map[int]*YakkMailBoxConnection
	conn         *websocket.Conn
	toCloseChan  chan struct{}
}

func InitYakkMailRoomConnection(mailRoomName string, ownerID int, conn *websocket.Conn, joinedRoomChan chan *YakkMailBoxConnection) (yakkmailRoom YakkMailRoomConnection) {
	mailRoom := YakkMailRoomConnection{
		Name:         mailRoomName,
		OwnerID:      ownerID,
		WriteChannel: make(chan yakkserver.YakkMailboxMessage),
		MailBoxes:    make(map[int]*YakkMailBoxConnection),
		conn:         conn,
	}
	go mailRoom.HandleMailRoomWrites()
	go mailRoom.HandleMailRoomReads(joinedRoomChan)
	return mailRoom
}

func (yakkMailRoom *YakkMailRoomConnection) CreateMailBox(recipientID int) *YakkMailBoxConnection {
	yakkMailBox := YakkMailBoxConnection{
		Name:         yakkMailRoom.Name,
		SenderID:     yakkMailRoom.OwnerID,
		RecipientID:  recipientID,
		WriteChannel: yakkMailRoom.WriteChannel,
		RecvChannel:  make(chan yakkserver.YakkMailboxMessage),
	}
	yakkMailRoom.MailBoxes[yakkMailBox.RecipientID] = &yakkMailBox
	return &yakkMailBox
}

func (yakkmailRoom *YakkMailRoomConnection) HandleMailRoomWrites() {
	for {
		mailBoxMsg, more := <-yakkmailRoom.WriteChannel
		if more {
			yakkmailRoom.conn.WriteJSON(mailBoxMsg)
		} else {
			break
		}
	}
}

func (yakkMailRoom *YakkMailRoomConnection) HandleMailRoomReads(joinedRoomChan chan *YakkMailBoxConnection) {
	for {

		select {
		case <-yakkMailRoom.toCloseChan:
			break // clean up this goroutine
		default:
		}

		var mailBoxMsg yakkserver.YakkMailboxMessage
		if err := yakkserver.ReadOneWS(yakkMailRoom.conn, &mailBoxMsg); err != nil {
			return
		}

		if _, ok := yakkMailRoom.MailBoxes[mailBoxMsg.Sender]; !ok {
			// mailbox does not exist. Create a new one?
			mailBox := yakkMailRoom.CreateMailBox(mailBoxMsg.Sender)
			joinedRoomChan <- mailBox
		}

		mailBox := yakkMailRoom.MailBoxes[mailBoxMsg.Sender] // this is safe because we just created it above if it doesn't exist
		mailBox.RecvChannel <- mailBoxMsg
	}
}

func (yakkmailRoom *YakkMailRoomConnection) Close() {
	// Close the write channel, close all mailboxes, then close the conn
	close(yakkmailRoom.WriteChannel)

	for mailboxID := range yakkmailRoom.MailBoxes {
		yakkmailRoom.toCloseChan <- struct{}{}
		delete(yakkmailRoom.MailBoxes, mailboxID)
	}

	yakkmailRoom.conn.Close()
	log.Info().Msg("Closing websocket...")
}

const (
	YAKK_UNINITIALISED       = "YAKK_UNINITIALISED"
	YAKK_INITIALISED         = "YAKK_INITIALISED"
	YAKK_OFFER_RECEIVED      = "YAKK_OFFER_RECEIVED"
	YAKK_WAIT_FOR_ANSWER     = "YAKK_WAIT_FOR_ANSWER"
	YAKK_WAIT_FOR_CONNECTION = "YAKK_WAIT_FOR_CONNECTION"
	YAKK_CONNECTED           = "YAKK_CONNECTED"
	// Encryption connection
	YAKK_EXCHANGINGPAKE  = "YAKK_EXCHANGINGPAKE"
	YAKK_EXCHANGINGPAKE2 = "YAKK_EXCHANGINGPAKE2"
)

const (
	// For client / server use

	// Schollz PAKE
	YAKKMSG_STARTPAKEEXCHANGE = "YAKKMSG_STARTPAKEEXCHANGE"
	YAKKMSG_PAKEEXCHANGE      = "YAKKMSG_PAKEEXCHANGE"

	// JPake
	YAKKMSG_JPAKEROUND1          = "YAKKMSG_JPAKEROUND1"
	YAKKMSG_JPAKEROUND2          = "YAKKMSG_JPAKEROUND2"
	YAKKMSG_JPAKEKEYCONFIRMATION = "YAKKMSG_JPAKEKEYCONFIRMATION"

	// WEBRTC Connections
	YAKKMSG_OFFER             = "YAKKMSG_OFFER"
	YAKKMSG_ANSWER            = "YAKKMSG_ANSWER"
	YAKKMSG_NEW_ICE_CANDIDATE = "YAKKMSG_NEW_ICE_CANDIDATE"
	YAKKMSG_CONNECTED         = "YAKKMSG_CONNECTED"

	// For signalling
	YAKKMSG_CREATEROOM   = "YAKKMSG_CREATEROOM"
	YAKKMSG_JOINROOM     = "YAKKMSG_JOINROOM"
	YAKKMSG_ROOMCREATED  = "YAKKMSG_ROOMCREATED"
	YAKKMSG_ROOMJOINED   = "YAKKMSG_ROOMJOINED"
	YAKKMSG_REQUESTOFFER = "YAKKMSG_REQUESTOFFER"

	YAKKMSG_ERROR        = "YAKKMSG_ERROR"
	YAKKMSG_DISCONNECTED = "YAKKMSG_DISCONNECTED"
)
