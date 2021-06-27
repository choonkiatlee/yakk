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

// Todo: Fact Check this
func StapleConnections(conn1 io.ReadWriteCloser, conn2 io.ReadWriteCloser) {
	// channels to wait on the close event for each connection
	conn1Closed := make(chan struct{}, 1)
	conn2Closed := make(chan struct{}, 1)

	go broker(conn1, conn2, conn2Closed)
	go broker(conn2, conn1, conn1Closed)

	// wait for one half of the proxy to exit, then trigger a shutdown of the
	// other half by calling CloseRead(). This will break the read loop in the
	// broker and allow us to fully close the connection cleanly without a
	// "use of closed network connection" error.
	var waitFor chan struct{}
	select {
	case <-conn2Closed:
		// the client closed first and any more packets from the server aren't
		// useful, so we can optionally SetLinger(0) here to recycle the port
		// faster.
		err := conn1.Close()
		if err != nil {
			panic(err)
		}
		waitFor = conn1Closed
	case <-conn1Closed:
		err := conn2.Close()
		if err != nil {
			panic(err)
		}
		waitFor = conn2Closed
	}

	// Wait for the other connection to close.
	// This "waitFor" pattern isn't required, but gives us a way to track the
	// connection and ensure all copies terminate correctly; we can trigger
	// stats on entry and deferred exit of this function.
	<-waitFor
	log.Info().Msg("Closed Stapled Connections.")
}

// This does the actual data transfer.
// The broker only closes the Read side.
func broker(dst io.ReadWriteCloser, src io.ReadWriteCloser, srcClosed chan struct{}) {
	// We can handle errors in a finer-grained manner by inlining io.Copy (it's
	// simple, and we drop the ReaderFrom or WriterTo checks for
	// net.Conn->net.Conn transfers, which aren't needed). This would also let
	// us adjust buffersize.
	// Create a buffer of 4KB. This is to prevent readwrite errors as in:
	// https://github.com/pion/datachannel/issues/59
	buf := make([]byte, 4<<10)
	_, err := io.CopyBuffer(dst, src, buf)

	if err != nil {
		// We should be able to fairly simply ignore this
		log.Info().Msgf("Copy error: %s\n", err)
	}
	if err := src.Close(); err != nil {
		log.Info().Msgf("Close error: %s\n", err)
	}
	srcClosed <- struct{}{}
}

func GetRandomOpenPort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return -1, err
	}
	port := listener.Addr().(*net.TCPAddr).Port

	if err := listener.Close(); err != nil {
		return -1, err
	}

	return port, nil
}
