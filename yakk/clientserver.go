package yakk

import (
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/pion/webrtc/v3"
	"github.com/rs/zerolog/log"
)

type DetachedDataChannelCallback func(io.ReadWriteCloser)
type DataChannelCallback func(*webrtc.DataChannel)

func EmptyOnDataChannelCallbackFn(datachannel io.ReadWriteCloser)          {}
func EmptyOnCommandChannelCloseCallbackFn(datachannel *webrtc.DataChannel) {}

func RegisterDataChannelFns(
	peerConnection *webrtc.PeerConnection,
	onDataConnFn DetachedDataChannelCallback,
	onCommandChannelFn DetachedDataChannelCallback,
) {
	peerConnection.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		log.Info().Msgf("New DataChannel %s %d\n", dataChannel.Label(), dataChannel.ID())

		// Register channel opening handling
		dataChannel.OnOpen(func() {
			log.Info().Msgf("Data channel '%s'-'%d' open.\n", dataChannel.Label(), dataChannel.ID())
			RawDC, err := dataChannel.Detach()
			if err != nil {
				panic(err)
			}
			if strings.HasPrefix(dataChannel.Label(), "DataConn") {
				onDataConnFn(RawDC)
			} else if strings.HasPrefix(dataChannel.Label(), "command") {
				onCommandChannelFn(RawDC)
			}
		})
	})
}

// Convenience method
func ConnectToTCP(ConnectionAddr string, peerConnection *webrtc.PeerConnection) {
	connectToTCPFn := func(datachannel io.ReadWriteCloser) {
		StapleDCToTCP(ConnectionAddr, datachannel)
	}
	RegisterDataChannelFns(peerConnection, connectToTCPFn, EmptyOnDataChannelCallbackFn)
}

func StapleDCToTCP(ConnectionAddr string, RawDC io.ReadWriteCloser) {

	TcpConn, err := net.Dial("tcp", ConnectionAddr)
	if err != nil {
		panic(err)
	}
	log.Info().Msgf("Stapled %s", TcpConn.RemoteAddr())
	StapleConnections(TcpConn, RawDC)
}

func ListenOnTCP(ConnectionAddr string, peerConnection *webrtc.PeerConnection) {
	// Listen on a TCP Port. Create a new datachannel for every new connection
	// on the TCP Port and staple the new datachannel onto the new connection
	l, err := net.Listen("tcp", ConnectionAddr)
	if err != nil {
		panic(err)
	}

	id := uint16(1)
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal().Msg(err.Error())
		}

		log.Info().Msgf("Connection from %s\n", conn.RemoteAddr())

		// Pick a random id out of the connMap
		// For now, we just use running numbers as a quick hack, which supports 65535 connections
		// Create a datachannel with label.
		dataChannel, err := peerConnection.CreateDataChannel(fmt.Sprintf("DataConn%d", id), nil)
		if err != nil {
			panic(err)
		}
		id += 1

		dataChannel.OnOpen(func() {
			log.Info().Msgf("Data channel '%s'-'%d' open.\n", dataChannel.Label(), dataChannel.ID())
			RawDC, err := dataChannel.Detach()
			if err != nil {
				panic(err)
			}
			StapleConnections(conn, RawDC)
		})
	}
}
