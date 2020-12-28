package yakk

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

var compress = false

// MustReadStdin blocks until input is received from stdin
func MustReadStdin() string {
	r := bufio.NewReader(os.Stdin)

	var in string
	for {
		var err error
		in, err = r.ReadString('\n')
		if err != io.EOF {
			if err != nil {
				panic(err)
			}
		}
		in = strings.TrimSpace(in)
		if len(in) > 0 {
			break
		}
	}

	fmt.Println("")

	return in
}

// Encode encodes the input in base64
// It can optionally zip the input before encoding
func EncodeObj(obj interface{}, encrypted bool, encryptionKey []byte) (string, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	if compress {
		b = zip(b)
	}
	return EncodeBytes(b, encrypted, encryptionKey)
}

func EncodeBytes(b []byte, encrypted bool, encryptionKey []byte) (string, error) {
	var err error = nil
	if encrypted {
		b, err = AESencrypt(b, encryptionKey)
	}
	return base64.StdEncoding.EncodeToString(b), err
}

// Decode decodes the input from base64
// It can optionally unzip the input after decoding
func DecodeObj(in string, obj interface{}, encrypted bool, encryptionKey []byte) error {
	b, err := DecodeBytes(in, encrypted, encryptionKey)
	if err != nil {
		return (err)
	}

	if compress {
		b = unzip(b)
	}

	err = json.Unmarshal(b, obj)
	if err != nil {
		return (err)
	}
	return nil
}

func DecodeBytes(in string, encrypted bool, encryptionKey []byte) ([]byte, error) {
	bytesFromMsg, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return []byte{}, err
	}
	if encrypted {
		bytesFromMsg, err = AESdecrypt(bytesFromMsg, encryptionKey)
		if err != nil {
			return []byte{}, err
		}
	}
	return bytesFromMsg, nil
}

func zip(in []byte) []byte {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, err := gz.Write(in)
	if err != nil {
		panic(err)
	}
	err = gz.Flush()
	if err != nil {
		panic(err)
	}
	err = gz.Close()
	if err != nil {
		panic(err)
	}
	return b.Bytes()
}

func unzip(in []byte) []byte {
	var b bytes.Buffer
	_, err := b.Write(in)
	if err != nil {
		panic(err)
	}
	r, err := gzip.NewReader(&b)
	if err != nil {
		panic(err)
	}
	res, err := ioutil.ReadAll(r)
	if err != nil {
		panic(err)
	}
	return res
}

func GetInputFromStdin(msg string) string {
	fmt.Println(msg)
	return MustReadStdin()
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
