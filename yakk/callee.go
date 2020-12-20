package yakk

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/choonkiatlee/yakk/yakkserver"
	"github.com/pion/webrtc/v3"
)

// Callee
// YAKK_UNINITIALISED -> YAKK_INITIALISED -> YAKK_OFFER_RECEIVED -> YAKK_CONNECTED

func Callee(roomID string) {

	if len(roomID) == 0 {
		fmt.Println("Input Mailbox Name: ")
		roomID = MustReadStdin()
	}

	fmt.Print("RoomID is: ", roomID)

	yakkMailBoxConnection, err := JoinMailBox(roomID)
	if err != nil {
		panic(err)
	}
	ws := yakkMailBoxConnection.Conn

	// When the callee gets his conn, send a message to the caller to indicate that
	// the callee is ready to begin.
	SendMail(yakkserver.YAKKMSG_REQUESTOFFER, base64.StdEncoding.EncodeToString(yakkMailBoxConnection.PakeObj.Bytes()), ws)

	var peerConnection *webrtc.PeerConnection
	state := YAKK_UNINITIALISED
	for {
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
		case yakkserver.YAKKMSG_PAKEEXCHANGE:
			if state == YAKK_UNINITIALISED {
				P_bytes, err := base64.StdEncoding.DecodeString(mailBoxMsg.Payload)
				if err != nil {
					panic(err)
				}
				err = yakkMailBoxConnection.PakeObj.Update(P_bytes)
				if err != nil {
					fmt.Println("Error: ", err)
					panic(err)
				}
				SendMail(yakkserver.YAKKMSG_PAKEEXCHANGE, base64.StdEncoding.EncodeToString(yakkMailBoxConnection.PakeObj.Bytes()), ws)
				fmt.Println(yakkMailBoxConnection.PakeObj.SessionKey())
				fmt.Println(yakkMailBoxConnection.PakeObj.IsVerified())
				state = YAKK_INITIALISED
			}
		case yakkserver.YAKKMSG_OFFER:
			if state == YAKK_INITIALISED {
				peerConnection, err = InitPeerConnection(&state, yakkMailBoxConnection)
				if err != nil {
					panic(err)
				}
				InitDataChannelCallee(peerConnection)
				answer, err := HandleOfferMsg(peerConnection, mailBoxMsg, yakkMailBoxConnection.PakeObj)
				if err != nil {
					panic(err)
				}
				SendMail(yakkserver.YAKKMSG_ANSWER, EncodeObj(answer, true, yakkMailBoxConnection.PakeObj), ws)
				state = YAKK_OFFER_RECEIVED
			}
		case yakkserver.YAKKMSG_NEW_ICE_CANDIDATE:
			if state == YAKK_OFFER_RECEIVED {
				var candidate webrtc.ICECandidateInit
				if err := DecodeObj(mailBoxMsg.Payload, &candidate, true, yakkMailBoxConnection.PakeObj); err != nil {
					panic(err)
				}
				if err := peerConnection.AddICECandidate(candidate); err != nil {
					panic(err)
				}
			}
		}
	}
	select {}

}
