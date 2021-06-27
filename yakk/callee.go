package yakk

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"
	"github.com/rs/zerolog/log"
	"github.com/sethvargo/go-diceware/diceware"
)

func Callee(yakkMailRoomConnection *YakkMailRoomConnection, pw []byte, host string, port string, connectedSuccessString string, wg *sync.WaitGroup) error {
	OperateMailRoom(yakkMailRoomConnection, pw, host, port, connectedSuccessString, wg)
	return nil
}

func newRoomName(collection *firestore.CollectionRef) (string, error) {
	randomWordsList, err := diceware.Generate(1)
	if err != nil {
		return "", err
	}
	roomName := strings.Join(randomWordsList, "_")
	doc, err := collection.Doc(roomName).Get(context.Background())

	if err == nil && doc.Exists() {
		return newRoomName(collection)
	} else {
		return roomName, nil
	}
}

func CreateMailRoom(yakkMailRoomConnection *YakkMailRoomConnection) (string, error) {

	collection := yakkMailRoomConnection.getMailRoomCollectionRef()
	roomName, err := newRoomName(collection)
	if err != nil {
		return "", err
	}

	ownerId := uuid.New().String()
	newMailRoom := map[string]interface{}{
		"RoomName": roomName,
		"OwnerId":  ownerId,
	}
	_, err = collection.Doc(roomName).Create(
		context.Background(), newMailRoom,
	)
	if err == nil {
		yakkMailRoomConnection.RoomName = roomName
		yakkMailRoomConnection.OwnerId = ownerId
	}
	return roomName, err
}

// Listen to the mail room and create mailboxes as needed.
func OperateMailRoom(yakkMailRoomConnection *YakkMailRoomConnection, pw []byte, host string, port string, connectedSuccessString string, wg *sync.WaitGroup) error {
	collection := yakkMailRoomConnection.getMailBoxCollectionRef()
	snapIter := collection.Snapshots(context.Background())
	defer snapIter.Stop()

	for {
		snap, err := snapIter.Next()
		if err != nil {
			fmt.Printf("Error in listening for msg. %s", err)
		}

		for _, change := range snap.Changes {
			switch change.Kind {
			case firestore.DocumentAdded:
				yakkMailBoxConnection := YakkMailBoxConnection{
					firebaseClient: yakkMailRoomConnection.firebaseClient,
					isOwner:        true,
					RoomName:       yakkMailRoomConnection.RoomName,
					MailBoxName:    change.Doc.Ref.ID,
					State:          YAKKSTATE_UNINITIALISED,
					RecvChan:       make(chan YakkMailBoxMsg),
				}
				log.Info().Msgf("Mailbox %s added", change.Doc.Ref.ID)
				go OperateMailBox(&yakkMailBoxConnection, pw, host, port, connectedSuccessString, wg)
			}
		}
	}
}

func OperateMailBox(yakkMailBoxConnection *YakkMailBoxConnection, pw []byte, host string, port string, connectedSuccessString string, wg *sync.WaitGroup) error {
	go yakkMailBoxConnection.ListenForMsgs()

	err := JPAKEExchange(pw, yakkMailBoxConnection, true)
	if err != nil {
		return err
	}
	peerConnection, err := CalleeInitialiseRTCConn(yakkMailBoxConnection, connectedSuccessString, wg)
	if err != nil {
		return err
	}
	fmt.Printf("Connected to: %s, listening on port: %s", yakkMailBoxConnection.MailBoxName, port)
	ConnectToTCP(net.JoinHostPort(host, port), peerConnection)
	return nil
}

func CalleeInitialiseRTCConn(yakkMailBoxConnection *YakkMailBoxConnection, connectedSuccessString string, wg *sync.WaitGroup) (*webrtc.PeerConnection, error) {
	var peerConnection *webrtc.PeerConnection
	var err error

HandleWSMsgLoop:
	for {
		mailBoxMsg := <-yakkMailBoxConnection.RecvChan
		switch mailBoxMsg.MsgType {
		case YAKKMSG_OFFER:
			if yakkMailBoxConnection.State == YAKKSTATE_INITIALISED {
				peerConnection, err = InitPeerConnection(yakkMailBoxConnection, wg)
				if err != nil {
					return &webrtc.PeerConnection{}, err
				}

				answer, err := HandleOfferMsg(peerConnection, mailBoxMsg, yakkMailBoxConnection.SessionKey)
				if err != nil {
					return &webrtc.PeerConnection{}, err
				}
				payload, err := EncodeObj(answer, true, yakkMailBoxConnection.SessionKey)
				if err != nil {
					return &webrtc.PeerConnection{}, err
				}
				yakkMailBoxConnection.SendMsg(
					YAKKMSG_ANSWER,
					payload,
				)
				yakkMailBoxConnection.State = YAKKSTATE_WAITFORCONNECTION
			}
		case YAKKMSG_NEW_ICE_CANDIDATE:
			if yakkMailBoxConnection.State == YAKKSTATE_WAITFORCONNECTION {
				err := HandleNewICEConnection(mailBoxMsg, peerConnection, yakkMailBoxConnection.SessionKey)
				if err != nil {
					return &webrtc.PeerConnection{}, err
				}
			}
		case YAKKMSG_CONNECTED:
			// Handle connection
			if yakkMailBoxConnection.State == YAKKSTATE_WAITFORCONNECTION {
				fmt.Println(connectedSuccessString)
				yakkMailBoxConnection.State = YAKKSTATE_CONNECTED
				break HandleWSMsgLoop
			}
		}
	}
	return peerConnection, nil
}
