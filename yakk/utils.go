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
