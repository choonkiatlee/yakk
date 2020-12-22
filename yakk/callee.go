package yakk

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/choonkiatlee/yakk/yakkserver"
	"github.com/pion/webrtc/v3"
)

// Callee
// YAKK_UNINITIALISED -> YAKK_INITIALISED -> YAKK_OFFER_RECEIVED -> YAKK_CONNECTED

func ConnectAsCallee(ConnectionAddr string, yakkMailBoxConnection YakkMailBoxConnection) {

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
				peerConnection, err = InitPeerConnection(&state, true, yakkMailBoxConnection)
				if err != nil {
					panic(err)
				}
				InitDataChannelCallee(ConnectionAddr, peerConnection)
				// _, err = InitDataChannelCallee("data", peerConnection)
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

func InitDataChannelCallee(ConnectionAddr string, peerConnection *webrtc.PeerConnection) {
	peerConnection.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", dataChannel.Label(), dataChannel.ID())

		// Register channel opening handling
		dataChannel.OnOpen(func() {
			fmt.Printf("Data channel '%s'-'%d' open.\n", dataChannel.Label(), dataChannel.ID())

			if strings.HasPrefix(dataChannel.Label(), "DataConn") {
				RawDC, err := dataChannel.Detach()
				if err != nil {
					panic(err)
				}
				ConnectToTCP(ConnectionAddr, RawDC)
			}
		})
	})
}

func ConnectToTCP(ConnectionAddr string, RawDC io.ReadWriteCloser) {
	// Connect to a tcp port and staple the datachannel onto the connection
	TcpConn, err := net.Dial("tcp", ConnectionAddr)
	if err != nil {
		panic(err)
	}
	StapleConnections(TcpConn, RawDC)
}

func Callee(ConnectionAddr string) {
	yakkMailBoxConnection, err := CreateMailBox()
	if err != nil {
		panic(err)
	}

	fmt.Println(fmt.Sprintf("Code is: %s", yakkMailBoxConnection.Name))
	fmt.Println(fmt.Sprintf("Connect as: yakk client %s -l <port>", yakkMailBoxConnection.Name))

	ConnectAsCallee(ConnectionAddr, yakkMailBoxConnection)
}
