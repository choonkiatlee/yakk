package yakk

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/pion/webrtc/v3"
	"github.com/rs/zerolog/log"
)

func Caller(yakkMailBoxConnection *YakkMailBoxConnection, mailRoomName string, pw []byte, connectedSuccessString string, wg *sync.WaitGroup) (*webrtc.PeerConnection, *webrtc.DataChannel, error) {
	err := createMailBox(yakkMailBoxConnection, mailRoomName)
	if err != nil {
		return &webrtc.PeerConnection{}, &webrtc.DataChannel{}, err
	}
	go yakkMailBoxConnection.ListenForMsgs()

	if len(pw) == 0 {
		pw = []byte(GetInputFromStdin("Input MailRoom PW: "))
	}
	err = JPAKEExchange(pw, yakkMailBoxConnection, true)
	retries := 0
	for err != nil && retries < 5 {
		pw = []byte(GetInputFromStdin("Input MailRoom PW: "))
		err = JPAKEExchange(pw, yakkMailBoxConnection, true)
		retries += 1
	}

	return CallerInitialiseRTCConn(yakkMailBoxConnection, connectedSuccessString, wg)
}

func createMailBox(yakkMailBoxConnection *YakkMailBoxConnection, mailRoomName string) error {

	yakkMailBoxConnection.RoomName = mailRoomName
	docRef := yakkMailBoxConnection.getMailRoomDocRef()
	snapshot, err := docRef.Get(
		context.Background(),
	)
	if err != nil {
		return err
	}

	yakkMailRoomSchema := YakkMailRoomSchema{}
	mapstructure.Decode(snapshot.Data(), &yakkMailRoomSchema)

	// todo: retry here till get a unique uuid...
	id := uuid.New().String()
	if yakkMailRoomSchema.MailBoxes == nil {
		yakkMailRoomSchema.MailBoxes = make(map[string]YakkMailBoxSchema)
	}

	yakkMailRoomSchema.MailBoxes[id] = YakkMailBoxSchema{}
	yakkMailBoxConnection.MailBoxName = id
	yakkMailBoxConnection.isOwner = false

	// docRef.Collection("MailBoxes").Doc(id).Create(context.Background(), YakkMailBoxSchema{})
	mailBoxDocRef := docRef.Collection("MailBoxes").Doc(id)
	mailBoxDocRef.Create(context.Background(), map[string]string{id: id})
	mailBoxDocRef.Collection("Msgs").Doc("OtoRMsg").Create(context.Background(), YakkMailBoxMsg{})
	mailBoxDocRef.Collection("Msgs").Doc("RtoOMsg").Create(context.Background(), YakkMailBoxMsg{})
	return err
}

func CallerInitialiseRTCConn(yakkMailBoxConnection *YakkMailBoxConnection, connectedSuccessString string, wg *sync.WaitGroup) (*webrtc.PeerConnection, *webrtc.DataChannel, error) {

	peerConnection, err := InitPeerConnection(yakkMailBoxConnection, wg)
	if err != nil {
		return &webrtc.PeerConnection{}, &webrtc.DataChannel{}, err
	}

	commandDataChannel, err := CreateCommandDataChannel(peerConnection)
	if err != nil {
		return &webrtc.PeerConnection{}, &webrtc.DataChannel{}, err
	}

	offer, err := HandleNegotiationNeededEvent(peerConnection)
	if err != nil {
		return &webrtc.PeerConnection{}, &webrtc.DataChannel{}, err
	}

	payload, err := EncodeObj(offer, true, yakkMailBoxConnection.SessionKey)
	if err != nil {
		return &webrtc.PeerConnection{}, &webrtc.DataChannel{}, err
	}

	yakkMailBoxConnection.SendMsg(
		YAKKMSG_OFFER,
		payload,
	)
	yakkMailBoxConnection.State = YAKKSTATE_WAITFORANSWER

HandleWSMsgLoop:
	for {
		mailBoxMsg := <-yakkMailBoxConnection.RecvChan
		switch mailBoxMsg.MsgType {
		case YAKKMSG_ANSWER:
			if yakkMailBoxConnection.State == YAKKSTATE_WAITFORANSWER {
				err = HandleAnswerMsg(mailBoxMsg, peerConnection, yakkMailBoxConnection.SessionKey)
				if err != nil {
					return &webrtc.PeerConnection{}, &webrtc.DataChannel{}, err
				}
				yakkMailBoxConnection.State = YAKKSTATE_WAITFORCONNECTION
			}
		case YAKKMSG_NEW_ICE_CANDIDATE:
			if (yakkMailBoxConnection.State == YAKKSTATE_WAITFORANSWER) || (yakkMailBoxConnection.State == YAKKSTATE_WAITFORCONNECTION) {
				err = HandleNewICEConnection(mailBoxMsg, peerConnection, yakkMailBoxConnection.SessionKey)
				if err != nil {
					return &webrtc.PeerConnection{}, &webrtc.DataChannel{}, err
				}
			}
		case YAKKMSG_CONNECTED:
			// Handle connection
			log.Debug().Msg("RTC Connected.")
			fmt.Println(connectedSuccessString)
			yakkMailBoxConnection.SendMsg(
				YAKKMSG_CONNECTED,
				"",
			)
			yakkMailBoxConnection.State = YAKKSTATE_CONNECTED
			break HandleWSMsgLoop
		}
	}
	return peerConnection, commandDataChannel, nil
}
