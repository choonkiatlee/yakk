package yakk

import (
	"fmt"
	"sync"

	"github.com/pion/webrtc/v3"
	"github.com/rs/zerolog/log"
)

// Callee
// YAKK_UNINITIALISED -> YAKK_INITIALISED -> YAKK_OFFER_RECEIVED -> YAKK_CONNECTED

func CalleeInitialiseRTCConn(yakkMailBoxConnection *YakkMailBoxConnection, wg *sync.WaitGroup) (*webrtc.PeerConnection, error) {
	var peerConnection *webrtc.PeerConnection
	var err error

HandleWSMsgLoop:
	for {
		mailBoxMsg := <-yakkMailBoxConnection.RecvChannel
		switch mailBoxMsg.Msg_type {
		case YAKKMSG_OFFER:
			if yakkMailBoxConnection.State == YAKK_INITIALISED {
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
				yakkMailBoxConnection.State = YAKK_WAIT_FOR_CONNECTION
			}
		case YAKKMSG_NEW_ICE_CANDIDATE:
			if yakkMailBoxConnection.State == YAKK_WAIT_FOR_CONNECTION {
				err := HandleNewICEConnection(mailBoxMsg, peerConnection, yakkMailBoxConnection.SessionKey)
				if err != nil {
					return &webrtc.PeerConnection{}, err
				}
			}
		case YAKKMSG_CONNECTED:
			// Handle connection
			log.Printf("Got connected msg.")
			if yakkMailBoxConnection.State == YAKK_WAIT_FOR_CONNECTION {
				yakkMailBoxConnection.State = YAKK_CONNECTED
				break HandleWSMsgLoop
			}
		}
	}
	log.Debug().Msg("RTC Connection Complete")
	return peerConnection, nil
}

func Callee(wg *sync.WaitGroup, pw []byte, clientCommandName string, keepAlive bool, signallingServerURL string) (*webrtc.PeerConnection, error) {

	joinedRoomChan := make(chan *YakkMailBoxConnection)
	yakkMailRoomConnection, err := CreateMailRoom(joinedRoomChan, keepAlive, signallingServerURL)
	if err != nil {
		return &webrtc.PeerConnection{}, err
	}

	fmt.Println(fmt.Sprintf("Your mailroom name is: %s", yakkMailRoomConnection.Name))

	// For JPake
	// Wait for someone to join the room
	if len(pw) == 0 {
		pw = []byte(GetInputFromStdin("Input MailRoom PW: "))
	}
	fmt.Println(fmt.Sprintf("One line connection command: yakk %s %s --pw %s", clientCommandName, yakkMailRoomConnection.Name, string(pw)))

	yakkMailBoxConnection := <-joinedRoomChan

	err = JPAKEExchange(pw, yakkMailBoxConnection, true)
	if err != nil {
		return &webrtc.PeerConnection{}, err
	}

	// For Schollz
	// err = SchollzCalleePAKEExchange(pw, &yakkMailBoxConnection)
	// if err != nil {
	// 	return &webrtc.PeerConnection{}, err
	// }

	peerConnection, err := CalleeInitialiseRTCConn(yakkMailBoxConnection, wg)
	return peerConnection, err
}
