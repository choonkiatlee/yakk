package yakk

import (
	"sync"

	"github.com/pion/webrtc/v3"
	"github.com/rs/zerolog/log"
)

func CallerInitialiseRTCConn(yakkMailBoxConnection *YakkMailBoxConnection, wg *sync.WaitGroup) (*webrtc.PeerConnection, *webrtc.DataChannel, error) {

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
	yakkMailBoxConnection.State = YAKK_WAIT_FOR_ANSWER

HandleWSMsgLoop:
	for {
		mailBoxMsg := <-yakkMailBoxConnection.RecvChannel
		switch mailBoxMsg.Msg_type {
		case YAKKMSG_ANSWER:
			if yakkMailBoxConnection.State == YAKK_WAIT_FOR_ANSWER {
				err = HandleAnswerMsg(mailBoxMsg, peerConnection, yakkMailBoxConnection.SessionKey)
				if err != nil {
					return &webrtc.PeerConnection{}, &webrtc.DataChannel{}, err
				}
				yakkMailBoxConnection.State = YAKK_WAIT_FOR_CONNECTION
			}
		case YAKKMSG_NEW_ICE_CANDIDATE:
			if (yakkMailBoxConnection.State == YAKK_WAIT_FOR_ANSWER) || (yakkMailBoxConnection.State == YAKK_WAIT_FOR_CONNECTION) {
				err = HandleNewICEConnection(mailBoxMsg, peerConnection, yakkMailBoxConnection.SessionKey)
			}
		case YAKKMSG_CONNECTED:
			// Handle connection
			log.Debug().Msg("RTC Connected, closing websocket...")
			yakkMailBoxConnection.SendMsg(
				YAKKMSG_CONNECTED,
				"",
			)
			yakkMailBoxConnection.State = YAKK_CONNECTED
			break HandleWSMsgLoop
		}
	}
	log.Debug().Msg("Connected...")
	return peerConnection, commandDataChannel, nil
}

func Caller(roomID string, pw []byte, wg *sync.WaitGroup) (*webrtc.PeerConnection, *webrtc.DataChannel, error) {

	if len(roomID) == 0 {
		roomID = GetInputFromStdin("Input MailRoom Name: ")
	}

	_, yakkMailBoxConnection, err := JoinMailRoom(roomID)
	if err != nil {
		return &webrtc.PeerConnection{}, &webrtc.DataChannel{}, err
	}
	// Send a request offer message over to the mailroom owner to tell him we want to call him
	yakkMailBoxConnection.SendMsg(
		YAKKMSG_REQUESTOFFER, "",
	)
	if len(pw) == 0 {
		pw = []byte(GetInputFromStdin("Input MailRoom PW: "))
	}

	// Use JPAKE
	err = JPAKEExchange(pw, yakkMailBoxConnection, true)
	if err != nil {
		return &webrtc.PeerConnection{}, &webrtc.DataChannel{}, err
	}

	// For schollz
	// pw = []byte("asdasd")
	// err = SchollzCallerPAKEExchange(pw, &yakkMailBoxConnection)
	// if err != nil {
	// 	return &webrtc.PeerConnection{}, &webrtc.DataChannel{}, err
	// }

	return CallerInitialiseRTCConn(yakkMailBoxConnection, wg)
}
