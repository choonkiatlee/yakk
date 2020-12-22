package yakk

import (
	"github.com/gorilla/websocket"
	"github.com/schollz/pake"
)

type YakkMailBoxConnection struct {
	Name    string
	PakeObj *pake.Pake
	Conn    *websocket.Conn
}

const (
	YAKK_UNINITIALISED   = "YAKK_UNINITIALISED"
	YAKK_EXCHANGINGPAKE  = "YAKK_EXCHANGINGPAKE"
	YAKK_INITIALISED     = "YAKK_INITIALISED"
	YAKK_OFFER_RECEIVED  = "YAKK_OFFER_RECEIVED"
	YAKK_WAIT_FOR_ANSWER = "YAKK_WAIT_FOR_ANSWER"
	YAKK_CONNECTED       = "YAKK_CONNECTED"
)

// // The caller starts the datachannel in our case
// func InitDataChannelCaller(dataChannelId string, peerConnection *webrtc.PeerConnection) (*webrtc.DataChannel, error) {
// 	// Create a datachannel with label 'data'
// 	dataChannel, err := peerConnection.CreateDataChannel(dataChannelId, nil)
// 	if err != nil {
// 		return &webrtc.DataChannel{}, err
// 	}

// 	dataChannel.OnOpen(func() {
// 		fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", dataChannel.Label(), dataChannel.ID())

// 		for range time.NewTicker(5 * time.Second).C {
// 			message := "asdasdasdasd"
// 			fmt.Printf("Sending '%s'\n", message)

// 			// Send the message as text
// 			sendTextErr := dataChannel.SendText(message)
// 			if sendTextErr != nil {
// 				panic(sendTextErr)
// 			}
// 		}
// 	})
// 	return dataChannel, nil
// }

// func InitDataChannelCaller(peerConnection *webrtc.PeerConnection) {
// 	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
// 		fmt.Printf("New DataChannel %s %d\n", d.Label(), d.ID())

// 		// Register channel opening handling
// 		d.OnOpen(func() {
// 			fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", d.Label(), d.ID())

// 			for range time.NewTicker(5 * time.Second).C {
// 				message := "asdasdasdasd"
// 				fmt.Printf("Sending '%s'\n", message)

// 				// Send the message as text
// 				sendTextErr := d.SendText(message)
// 				if sendTextErr != nil {
// 					panic(sendTextErr)
// 				}
// 			}
// 		})

// 		// Register text message handling
// 		d.OnMessage(func(msg webrtc.DataChannelMessage) {
// 			fmt.Printf("Message from DataChannel '%s': '%s'\n", d.Label(), string(msg.Data))
// 		})
// 	})
// }
