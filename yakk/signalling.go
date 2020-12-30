package yakk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/choonkiatlee/yakk/yakkserver"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"github.com/rs/zerolog/log"
)

func SendDataChannelMail(msg_type, payload string, datachannel io.ReadWriteCloser) error {
	msg := yakkserver.YakkMailboxMessage{
		Msg_type: msg_type,
		Payload:  payload,
	}
	msgInBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = datachannel.Write(msgInBytes)
	return err
}

func CreateMailRoom(joinedRoomChan chan *YakkMailBoxConnection, keepAlive bool, signallingServerURL string) (YakkMailRoomConnection, error) {
	url := fmt.Sprintf("ws://%s/ws/?keepAlive=%t", signallingServerURL, keepAlive)
	log.Debug().Msg(url)
	// We force tcp4 because of a bug on android which doesn't resolve ipv6 addresses properly / only tries the ipv6 address, which heroku doesn't like?
	// This forces the dial to hit only ipv4 address resolution.
	tcp4OnlyDialer := websocket.DefaultDialer
	tcp4OnlyDialer.NetDial = func(network, addr string) (net.Conn, error) {
		return net.Dial("tcp4", addr)
	}
	conn, _, err := tcp4OnlyDialer.Dial(url, nil)
	if err != nil {
		log.Error().Msg(err.Error())
		return YakkMailRoomConnection{}, err
	}

	var mailBoxMsg yakkserver.YakkMailboxMessage
	err = yakkserver.ReadOneWS(conn, &mailBoxMsg)

	if mailBoxMsg.Msg_type == YAKKMSG_ERROR {
		log.Error().Msg("Signalling Server Error")
		panic(errors.New(mailBoxMsg.Payload))
	}

	yakkMailRoom := InitYakkMailRoomConnection(mailBoxMsg.Payload, mailBoxMsg.Recipient, conn, joinedRoomChan)

	return yakkMailRoom, nil
}

func JoinMailRoom(roomID string, signallingServerURL string) (YakkMailRoomConnection, *YakkMailBoxConnection, error) {

	url := fmt.Sprintf("ws://%s/ws/?roomID=%s", signallingServerURL, roomID)
	tcp4OnlyDialer := websocket.DefaultDialer
	tcp4OnlyDialer.NetDial = func(network, addr string) (net.Conn, error) {
		return net.Dial("tcp4", addr)
	}
	conn, _, err := tcp4OnlyDialer.Dial(url, nil)
	if err != nil {
		return YakkMailRoomConnection{}, &YakkMailBoxConnection{}, err
	}

	// Retry until the joinroom signal succeeds.
	mailBoxMsg := yakkserver.YakkMailboxMessage{
		Msg_type: YAKKMSG_ERROR,
		Payload:  "",
	}
	for mailBoxMsg.Msg_type == YAKKMSG_ERROR {
		if err := yakkserver.ReadOneWS(conn, &mailBoxMsg); err != nil {
			return YakkMailRoomConnection{}, &YakkMailBoxConnection{}, err
		}
		if mailBoxMsg.Msg_type == YAKKMSG_ERROR {
			fmt.Println("The mail box you have specified does not exist. Did you type it correctly?")
			roomID = GetInputFromStdin("Input MailRoom Name: ")
			conn.WriteJSON(yakkserver.YakkMailboxMessage{
				Msg_type: YAKKMSG_JOINROOM,
				Payload:  roomID,
			})
		} else {
			break
		}
	}
	yakkMailRoomConnection := InitYakkMailRoomConnection(roomID, mailBoxMsg.Recipient, conn, make(chan *YakkMailBoxConnection)) // we ignore the joinroom chan here as we setup our own mailbox
	yakkMailBox := yakkMailRoomConnection.CreateMailBox(mailBoxMsg.Sender)
	return yakkMailRoomConnection, yakkMailBox, err
}

func InitPeerConnection(yakkMailBoxConnection *YakkMailBoxConnection, wg *sync.WaitGroup) (*webrtc.PeerConnection, error) {
	// Prepare the configuration
	// Since this behavior diverges from the WebRTC API it has to be
	// enabled using a settings engine. Mixing both detached and the
	// OnMessage DataChannel API is not supported.
	// See: https://github.com/pion/webrtc/blob/master/examples/data-channels-detach/main.go

	// Create a SettingEngine and enable Detach
	s := webrtc.SettingEngine{}
	s.DetachDataChannels()

	// Create an API object with the engine
	api := webrtc.NewAPI(webrtc.WithSettingEngine(s))

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		return &webrtc.PeerConnection{}, err
	}

	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		desc := peerConnection.RemoteDescription()
		if desc != nil {
			payload, err := EncodeObj(c.ToJSON(), true, yakkMailBoxConnection.SessionKey)
			if err != nil {
				return
			}
			yakkMailBoxConnection.SendMsg(
				YAKKMSG_NEW_ICE_CANDIDATE,
				payload,
			)
		}
	})

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Info().Msgf("ICE Connection State has changed: %s\n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			yakkMailBoxConnection.SendMsg(
				YAKKMSG_CONNECTED,
				"",
			)
			log.Debug().Msg("Sending connected status...")
			wg.Add(1)
		}
		if connectionState == webrtc.ICEConnectionStateDisconnected {
			// to do: make a way to clean up the mailbox
			wg.Done()
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

func HandleOfferMsg(peerConnection *webrtc.PeerConnection, yakkMailBoxMsg yakkserver.YakkMailboxMessage, encryptionKey []byte) (webrtc.SessionDescription, error) {
	var offer webrtc.SessionDescription
	if err := DecodeObj(yakkMailBoxMsg.Payload, &offer, true, encryptionKey); err != nil {
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

func HandleAnswerMsg(yakkMailBoxMsg yakkserver.YakkMailboxMessage, peerConnection *webrtc.PeerConnection, encryptionKey []byte) error {

	var answer webrtc.SessionDescription
	if err := DecodeObj(yakkMailBoxMsg.Payload, &answer, true, encryptionKey); err != nil {
		return err
	}

	// Set Remote description to tell WebRTC about remote config
	if err := peerConnection.SetRemoteDescription(answer); err != nil {
		return err
	}

	return nil
}

func HandleNewICEConnection(yakkMailBoxMsg yakkserver.YakkMailboxMessage, peerConnection *webrtc.PeerConnection, encryptionKey []byte) error {
	var candidate webrtc.ICECandidateInit
	if err := DecodeObj(yakkMailBoxMsg.Payload, &candidate, true, encryptionKey); err != nil {
		return err
	}
	if err := peerConnection.AddICECandidate(candidate); err != nil {
		return err
	}
	return nil
}

// The caller starts the datachannel in our case
func CreateCommandDataChannel(peerConnection *webrtc.PeerConnection) (*webrtc.DataChannel, error) {
	// Create a datachannel with label.
	dataChannel, err := peerConnection.CreateDataChannel("command", nil)
	if err != nil {
		return &webrtc.DataChannel{}, err
	}

	dataChannel.OnOpen(func() {
		log.Info().Msgf("Command channel '%s'-'%d' open. ", dataChannel.Label(), dataChannel.ID())
	})
	return dataChannel, nil
}
