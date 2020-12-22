package yakk

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/schollz/pake"
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
func EncodeObj(obj interface{}, encrypted bool, pakeObj *pake.Pake) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	if compress {
		b = zip(b)
	}
	return EncodeBytes(b, encrypted, pakeObj)
}

func EncodeBytes(b []byte, encrypted bool, pakeObj *pake.Pake) string {
	if encrypted {
		key, err := GetEncryptionKey(pakeObj)
		if err != nil {
			panic(err)
		}
		b = encrypt(b, key)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// Decode decodes the input from base64
// It can optionally unzip the input after decoding
func DecodeObj(in string, obj interface{}, encrypted bool, pakeObj *pake.Pake) error {
	b, err := DecodeBytes(in, encrypted, pakeObj)
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

func DecodeBytes(in string, encrypted bool, pakeObj *pake.Pake) ([]byte, error) {
	bytesFromMsg, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return []byte{}, err
	}
	if encrypted {
		key, err := GetEncryptionKey(pakeObj)
		if err != nil {
			return []byte{}, err
		}
		bytesFromMsg = decrypt(bytesFromMsg, key)
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

// From https://www.melvinvivas.com/how-to-encrypt-and-decrypt-data-using-aes/
// To check
func encrypt(plaintext []byte, key []byte) (encrypted []byte) {

	//Since the key is in string, we need to convert decode it to bytes
	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}

	//Create a new GCM - https://en.wikipedia.org/wiki/Galois/Counter_Mode
	//https://golang.org/pkg/crypto/cipher/#NewGCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	//Create a nonce. Nonce should be from GCM
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}

	//Encrypt the data using aesGCM.Seal
	//Since we don't want to save the nonce somewhere else in this case, we add it as a prefix to the encrypted data. The first nonce argument in Seal is the prefix.
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext
}

func decrypt(encrypted []byte, key []byte) (decrypted []byte) {

	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}

	//Create a new GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	//Get the nonce size
	nonceSize := aesGCM.NonceSize()

	//Extract the nonce from the encrypted data
	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]

	//Decrypt the data
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		panic(err.Error())
	}

	return plaintext
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
		log.Printf("Copy error: %s\n", err)
	}
	if err := src.Close(); err != nil {
		log.Printf("Close error: %s\n", err)
	}
	srcClosed <- struct{}{}
}

// Todo: Fact Check this
func StapleConnections2(conn1 io.ReadWriteCloser, conn2 io.ReadWriteCloser, keepConn2Alive bool) {
	// channels to wait on the close event for each connection
	brokerError := make(chan struct{}, 1)

	go broker(conn1, conn2, brokerError)
	go broker(conn2, conn1, brokerError)

	<-brokerError
	conn1.Close()
	fmt.Println("Closing conn1...")

	// wait for one half of the proxy to exit, then trigger a shutdown of the
	// other half by calling CloseRead(). This will break the read loop in the
	// broker and allow us to fully close the connection cleanly without a
	// "use of closed network connection" error.
	// var waitFor chan struct{}
	// select {
	// case <-conn2Closed:
	// 	// the client closed first and any more packets from the server aren't
	// 	// useful, so we can optionally SetLinger(0) here to recycle the port
	// 	// faster.
	// 	err := conn1.Close()
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	waitFor = conn1Closed
	// case <-conn1Closed:
	// 	err := conn2.Close()
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	waitFor = conn2Closed
	// }

	// // Wait for the other connection to close.
	// // This "waitFor" pattern isn't required, but gives us a way to track the
	// // connection and ensure all copies terminate correctly; we can trigger
	// // stats on entry and deferred exit of this function.
	// <-waitFor
}
