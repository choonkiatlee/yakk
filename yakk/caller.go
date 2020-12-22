package yakk

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/choonkiatlee/yakk/yakkserver"
	"github.com/pion/webrtc/v3"
	"github.com/schollz/pake"
)

// Caller
// YAKK_UNINITIALISED -> YAKK_EXCHANGINGPAKE -> YAKK_INITIALISED -> YAKK_WAIT_FOR_ANSWER -> YAKK_CONNECTED

func ConnectAsCaller(ConnectionAddr string, yakkMailBoxConnection YakkMailBoxConnection) {
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
				peerConnection, err = InitPeerConnection(&state, false, yakkMailBoxConnection)
				if err != nil {
					panic(err)
				}
				// Open a datachannel
				// InitDataChannelCaller(peerConnection)
				// Commands between the caller and callee can be transmitted using this.
				// This is also used to keep the webrtc connection alive(?)
				_, err = CreateCommandDataChannel(peerConnection)

				// Start listening on TCP connection
				go ListenOnTCP(ConnectionAddr, peerConnection)

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

// The caller starts the datachannel in our case
func CreateCommandDataChannel(peerConnection *webrtc.PeerConnection) (*webrtc.DataChannel, error) {
	// Create a datachannel with label.
	dataChannel, err := peerConnection.CreateDataChannel("command", nil)
	if err != nil {
		return &webrtc.DataChannel{}, err
	}

	dataChannel.OnOpen(func() {
		fmt.Printf("Command channel '%s'-'%d' open. ", dataChannel.Label(), dataChannel.ID())
	})
	return dataChannel, nil
}

func ListenOnTCP(ConnectionAddr string, peerConnection *webrtc.PeerConnection) {
	// Listen on a TCP Port. Create a new datachannel for every new connection
	// on the TCP Port and staple the new datachannel onto the new connection
	l, err := net.Listen("tcp", ConnectionAddr)
	if err != nil {
		// fmt.Fprintln(os.Stderr, err)
		// os.Exit(1)
	}

	id := uint16(1)
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Connection from %s\n", conn.RemoteAddr())

		// Pick a random id out of the connMap
		// For now, we just use running numbers as a quick hack, which supports 65535 connections
		// Create a datachannel with label.
		dataChannel, err := peerConnection.CreateDataChannel(fmt.Sprintf("DataConn%d", id), nil)
		if err != nil {
			panic(err)
		}
		id += 1

		dataChannel.OnOpen(func() {
			fmt.Printf("Data channel '%s'-'%d' open.\n", dataChannel.Label(), dataChannel.ID())
			RawDC, err := dataChannel.Detach()
			if err != nil {
				panic(err)
			}
			StapleConnections(conn, RawDC)
		})
	}

}

func GetRoomIDFromStdin() string {
	fmt.Println("Input Mailbox Name: ")
	return MustReadStdin()
}

func Caller(roomID string, ConnectionAddr string) {

	if len(roomID) == 0 {
		roomID = GetRoomIDFromStdin()
	}

	yakkMailBoxConnection, err := JoinMailBox(roomID)
	if err != nil {
		panic(err)
	}

	ConnectAsCaller(ConnectionAddr, yakkMailBoxConnection)
	select {}
}
