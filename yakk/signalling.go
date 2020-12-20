// Handles all signalling and connection related stuff

package yakk

import (
	"crypto/elliptic"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/choonkiatlee/yakk/yakkserver"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"github.com/schollz/pake"
)

func SendMail(msg_type string, payload string, ws *websocket.Conn) error {
	// Send our answer back to the caller as json
	newOfferMsg := yakkserver.YakkMailboxMessage{
		Msg_type: msg_type,
		Payload:  payload,
	}
	return ws.WriteJSON(newOfferMsg)
}

var curve = elliptic.P256()

func CreateMailBox() (mailbox YakkMailBoxConnection, err error) {
	conn, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:6006/ws/", nil)
	if err != nil {
		return YakkMailBoxConnection{}, err
	}

	_, r, err := conn.NextReader()
	if err != nil {
		return YakkMailBoxConnection{}, err
	}
	msg, err := ioutil.ReadAll(r)

	var mailBoxMsg yakkserver.YakkMailboxMessage
	err = json.Unmarshal(msg, &mailBoxMsg)

	P, err := pake.Init([]byte(mailBoxMsg.Payload), 1, curve, 50*time.Millisecond)
	yakkMailBoxConnection := YakkMailBoxConnection{
		Name:    mailBoxMsg.Payload,
		PakeObj: P,
		Conn:    conn,
	}
	return yakkMailBoxConnection, nil
}

func JoinMailBox(roomID string) (mailbox YakkMailBoxConnection, err error) {
	conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("%s%s", "ws://127.0.0.1:6006/ws/", roomID), nil)
	if err != nil {
		return YakkMailBoxConnection{}, err
	}
	// The callee kicks off the pake by sending its bytes over to the caller, so it is the sender
	Q, err := pake.Init([]byte(roomID), 0, curve, 50*time.Millisecond) // This generates a 256-bit key
	return YakkMailBoxConnection{
		Name:    roomID,
		PakeObj: Q,
		Conn:    conn,
	}, err
}

func InitPeerConnection(state *string, yakkMailBoxConnection YakkMailBoxConnection) (*webrtc.PeerConnection, error) {
	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return &webrtc.PeerConnection{}, err
	}

	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		desc := peerConnection.RemoteDescription()
		if desc != nil {
			SendMail(yakkserver.YAKKMSG_NEW_ICE_CANDIDATE, EncodeObj(c.ToJSON(), true, yakkMailBoxConnection.PakeObj), yakkMailBoxConnection.Conn)
		}
	})

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			*state = YAKK_CONNECTED
			yakkMailBoxConnection.Conn.Close() // To do: what about multiple connections, etc. ?
		}
	})

	return peerConnection, nil
}

func HandleNegotiationNeededEvent(peerConnection *webrtc.PeerConnection) (webrtc.SessionDescription, error) {
	// Create an offer to send to the other process
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	// Note: this will start the gathering of ICE candidates
	if err = peerConnection.SetLocalDescription(offer); err != nil {
		return webrtc.SessionDescription{}, err
	}
	return offer, nil
}

func HandleOfferMsg(peerConnection *webrtc.PeerConnection, yakkMailBoxMsg yakkserver.YakkMailboxMessage, pakeObj *pake.Pake) (webrtc.SessionDescription, error) {
	var offer webrtc.SessionDescription
	if err := DecodeObj(yakkMailBoxMsg.Payload, &offer, true, pakeObj); err != nil {
		return webrtc.SessionDescription{}, err
	}

	// Set Remote description to tell WebRTC about remote config
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		return webrtc.SessionDescription{}, err
	}

	// Create an answer to send to the other process
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}
	return answer, nil
}

func HandleAnswerMsg(yakkMailBoxMsg yakkserver.YakkMailboxMessage, peerConnection *webrtc.PeerConnection, pakeObj *pake.Pake) error {

	var answer webrtc.SessionDescription
	if err := DecodeObj(yakkMailBoxMsg.Payload, &answer, true, pakeObj); err != nil {
		return err
	}

	// Set Remote description to tell WebRTC about remote config
	if err := peerConnection.SetRemoteDescription(answer); err != nil {
		return err
	}

	return nil
}

func GetEncryptionKey(PakeObj *pake.Pake) ([]byte, error) {
	// if !PakeObj.IsVerified() {
	// 	fmt.Println("The PAKE has not been verified to be safe. Are you sure you want to continue? Press y to agree")
	// 	if MustReadStdin() != "y" {
	// 		return []byte{}, errors.New("Manually Aborted because PAKE was not safe")
	// 	}
	// }
	key, err := PakeObj.SessionKey()
	if err != nil {
		return []byte{}, err
	}
	return key, nil
}
