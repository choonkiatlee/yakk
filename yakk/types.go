package yakk

import (
	"cloud.google.com/go/firestore"
)

type YakkMailBoxConnection struct {
	RoomName       string
	MailBoxName    string
	State          string
	SessionKey     []byte
	isOwner        bool // Is the user the mail room owner?
	RecvChan       chan YakkMailBoxMsg
	firebaseClient *firestore.Client
}

type YakkMailRoomConnection struct {
	RoomName       string
	OwnerId        string
	firebaseClient *firestore.Client
	MailBoxes      map[string]*YakkMailBoxConnection
}

type YakkMailRoomSchema struct {
	Name      string
	OwnerId   string
	MailBoxes map[string]YakkMailBoxSchema
}

type YakkMailBoxSchema struct {
	OtoRMsg YakkMailBoxMsg // Message from the mail room owner to the mail box recipient
	RtoOMsg YakkMailBoxMsg // Message from the mailbox recipient to the mail room owner
}

type YakkMailBoxMsg struct {
	MsgType string
	Payload string
	// Sender    string
	// Recipient string
}

const (
	// For client / server use
	// JPake
	YAKKMSG_JPAKEROUND1          = "JPAKEROUND1"
	YAKKMSG_JPAKEROUND2          = "JPAKEROUND2"
	YAKKMSG_JPAKEKEYCONFIRMATION = "JPAKEKEYCONFIRMATION"

	// WEBRTC Connections
	YAKKMSG_OFFER             = "OFFER"
	YAKKMSG_ANSWER            = "ANSWER"
	YAKKMSG_NEW_ICE_CANDIDATE = "NEW_ICE_CANDIDATE"
	YAKKMSG_CONNECTED         = "CONNECTED"

	// For signalling
	YAKKMSG_CREATEROOM   = "CREATEROOM"
	YAKKMSG_JOINROOM     = "JOINROOM"
	YAKKMSG_ROOMCREATED  = "ROOMCREATED"
	YAKKMSG_ROOMJOINED   = "ROOMJOINED"
	YAKKMSG_REQUESTOFFER = "REQUESTOFFER"

	YAKKMSG_ERROR        = "ERROR"
	YAKKMSG_DISCONNECTED = "DISCONNECTED"
)

const (
	YAKKSTATE_UNINITIALISED   = "UNINITIALISED"
	YAKKSTATE_EXCHANGINGPAKE  = "EXCHANGINGPAKE"
	YAKKSTATE_EXCHANGINGPAKE2 = "EXCHANGINGPAKE2"
	YAKKSTATE_INITIALISED     = "INITIALISED"

	YAKKSTATE_WAITFORANSWER     = "WAITFORANSWER"
	YAKKSTATE_WAITFORCONNECTION = "WAITFORCONNECTION"
	YAKKSTATE_CONNECTED         = "CONNECTED"
)
