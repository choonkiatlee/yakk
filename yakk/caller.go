package yakk

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/choonkiatlee/yakk/yakkserver"
	"github.com/pion/webrtc/v3"
	"github.com/schollz/pake"
)

// Caller
// YAKK_UNINITIALISED -> YAKK_EXCHANGINGPAKE -> YAKK_INITIALISED -> YAKK_WAIT_FOR_ANSWER -> YAKK_CONNECTED

func ConnectAsCaller(yakkMailBoxConnection YakkMailBoxConnection) {
	ws := yakkMailBoxConnection.Conn
	// Start the signalling process
	var peerConnection *webrtc.PeerConnection
	state := YAKK_UNINITIALISED
	for {
		// If we cannot read from the websocket, we assume it has been closed upon successful webrtc connection, and close this?
		_, msg, err := ws.ReadMessage()
		if err != nil {
			if state == YAKK_CONNECTED {
				break
			} else {
				panic(err)
			}
		}

		var mailBoxMsg yakkserver.YakkMailboxMessage
		err = json.Unmarshal(msg, &mailBoxMsg)
		if err != nil {
			panic(err)
		}

		switch mailBoxMsg.Msg_type {
		case yakkserver.YAKKMSG_REQUESTOFFER:
			if state == YAKK_UNINITIALISED {
				P_bytes, err := base64.StdEncoding.DecodeString(mailBoxMsg.Payload)
				if err != nil {
					panic(err)
				}
				err = yakkMailBoxConnection.PakeObj.Update(P_bytes)
				if err != nil {
					panic(err)
				}
				msg := EncodeBytes(yakkMailBoxConnection.PakeObj.Bytes(), false, &pake.Pake{})
				SendMail(yakkserver.YAKKMSG_PAKEEXCHANGE, msg, ws)
				state = YAKK_EXCHANGINGPAKE
			}
		case yakkserver.YAKKMSG_PAKEEXCHANGE:
			if state == YAKK_EXCHANGINGPAKE {
				P_bytes, err := base64.StdEncoding.DecodeString(mailBoxMsg.Payload)
				if err != nil {
					panic(err)
				}
				err = yakkMailBoxConnection.PakeObj.Update(P_bytes)
				if err != nil {
					panic(err)
				}
				// Key exchange finished. Now everyone has a sessionkey object
				fmt.Println(yakkMailBoxConnection.PakeObj.SessionKey())
				peerConnection, err = InitPeerConnection(&state, yakkMailBoxConnection)
				if err != nil {
					panic(err)
				}
				// Open a datachannel
				_, err = InitDataChannelCaller("data", peerConnection)
				offer, err := HandleNegotiationNeededEvent(peerConnection)
				if err != nil {
					panic(err)
				}
				SendMail(yakkserver.YAKKMSG_OFFER, EncodeObj(offer, true, yakkMailBoxConnection.PakeObj), ws)
				state = YAKK_WAIT_FOR_ANSWER
			}
		case yakkserver.YAKKMSG_NEW_ICE_CANDIDATE:
			if state == YAKK_WAIT_FOR_ANSWER {
				var candidate webrtc.ICECandidateInit
				if err := DecodeObj(mailBoxMsg.Payload, &candidate, true, yakkMailBoxConnection.PakeObj); err != nil {
					panic(err)
				}
				if err := peerConnection.AddICECandidate(candidate); err != nil {
					panic(err)
				}
			}
		case yakkserver.YAKKMSG_ANSWER:
			if state == YAKK_WAIT_FOR_ANSWER {
				err = HandleAnswerMsg(mailBoxMsg, peerConnection, yakkMailBoxConnection.PakeObj)
				if err != nil {
					panic(err)
				}
			}
		}
	}
}

func Caller() {
	yakkMailBoxConnection, err := CreateMailBox()
	if err != nil {
		panic(err)
	}

	fmt.Println(fmt.Sprintf("Code is: %s", yakkMailBoxConnection.Name))
	fmt.Println(fmt.Sprintf("Connect as: yakk client %s -l <port>", yakkMailBoxConnection.Name))

	ConnectAsCaller(yakkMailBoxConnection)
	select {}
}
