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
)

var compress = false

func PanicOnErr(err error) {
	if err != nil {
		panic(err)
	}
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

// From https://www.melvinvivas.com/how-to-encrypt-and-decrypt-data-using-aes/
// To check
func AESencrypt(plaintext []byte, key []byte) (encrypted []byte, err error) {

	//Since the key is in string, we need to convert decode it to bytes
	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		return []byte{}, err
	}

	//Create a new GCM - https://en.wikipedia.org/wiki/Galois/Counter_Mode
	//https://golang.org/pkg/crypto/cipher/#NewGCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return []byte{}, err
	}

	//Create a nonce. Nonce should be from GCM
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return []byte{}, err
	}

	//Encrypt the data using aesGCM.Seal
	//Since we don't want to save the nonce somewhere else in this case, we add it as a prefix to the encrypted data. The first nonce argument in Seal is the prefix.
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func AESdecrypt(encrypted []byte, key []byte) (decrypted []byte, err error) {

	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		return []byte{}, err
	}

	//Create a new GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return []byte{}, err
	}

	//Get the nonce size
	nonceSize := aesGCM.NonceSize()

	//Extract the nonce from the encrypted data
	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]

	//Decrypt the data
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return []byte{}, err
	}

	return plaintext, nil
}

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

func GetInputFromStdin(msg string) string {
	fmt.Println(msg)
	return MustReadStdin()
}
