package yakk

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"time"

	"github.com/choonkiatlee/jpake-go"
	"github.com/rs/zerolog/log"
	"github.com/schollz/pake"
)

func JPAKEExchange(pw []byte, yakkMailBoxConnection *YakkMailBoxConnection, checkSessionKey bool) error {
	log.Debug().Msgf("JPake Exchange Start between ids: %d, %d", yakkMailBoxConnection.SenderID, yakkMailBoxConnection.RecipientID)
	// From the perspective of Alice, sending to Bob. The protocol is written to be symmetric.
	log.Info().Msgf("Shared PW is: %s", pw)
	pakeObj, err := jpake.Init(string(pw))
	if err != nil {
		return err
	}

	// Send over the first message
	aliceFirstRoundMsg, err := pakeObj.GetRound1Message()
	yakkMailBoxConnection.SendMsg(
		YAKKMSG_JPAKEROUND1,
		base64.StdEncoding.EncodeToString(aliceFirstRoundMsg),
	)
	log.Debug().Msg("Sent First Round Msg")

HandleWSMsgLoop:
	for {
		mailBoxMsg := <-yakkMailBoxConnection.RecvChannel
		log.Debug().Msgf("Received Msg of type: %s", mailBoxMsg.Msg_type)
		switch mailBoxMsg.Msg_type {
		case YAKKMSG_JPAKEROUND1:
			bobFirstRoundMsg, err := base64.StdEncoding.DecodeString(mailBoxMsg.Payload)
			if err != nil {
				return err
			}
			aliceSecondRoundMsg, err := pakeObj.GetRound2Message([]byte(bobFirstRoundMsg))
			if err != nil {
				return err
			}
			yakkMailBoxConnection.SendMsg(
				YAKKMSG_JPAKEROUND2,
				base64.StdEncoding.EncodeToString(aliceSecondRoundMsg),
			)
			yakkMailBoxConnection.State = YAKK_EXCHANGINGPAKE
		case YAKKMSG_JPAKEROUND2:
			if yakkMailBoxConnection.State == YAKK_EXCHANGINGPAKE {
				bobSecondRoundMsg, err := base64.StdEncoding.DecodeString(mailBoxMsg.Payload)
				if err != nil {
					return err
				}
				aliceSharedKey, err := pakeObj.ComputeSharedKey([]byte(bobSecondRoundMsg))
				if err != nil {
					return err
				}
				// Check Session Key
				if checkSessionKey {

					aliceCheckSessionKeyMsg, err := pakeObj.ComputeCheckSessionKeyMsg()
					if err != nil {
						return err
					}

					yakkMailBoxConnection.SendMsg(
						YAKKMSG_JPAKEKEYCONFIRMATION,
						base64.StdEncoding.EncodeToString(aliceCheckSessionKeyMsg),
					)
					yakkMailBoxConnection.State = YAKK_EXCHANGINGPAKE2
				} else {
					yakkMailBoxConnection.SessionKey = aliceSharedKey
					yakkMailBoxConnection.State = YAKK_INITIALISED
					break HandleWSMsgLoop
				}
			}
		case YAKKMSG_JPAKEKEYCONFIRMATION:
			if yakkMailBoxConnection.State == YAKK_EXCHANGINGPAKE2 {
				bobCheckSessionKeyMsg, err := base64.StdEncoding.DecodeString(mailBoxMsg.Payload)
				if err != nil {
					return err
				}
				verified := pakeObj.CheckReceivedSessionKeyMsg([]byte(bobCheckSessionKeyMsg))
				if verified {
					yakkMailBoxConnection.SessionKey, err = pakeObj.SessionKey()
					yakkMailBoxConnection.State = YAKK_INITIALISED
					break HandleWSMsgLoop
				} else {
					return errors.New("Could not verify the session key!")
				}
			}
		}
	}
	log.Info().Msg("Completed PAKE Exchange")
	return nil
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

func PanicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}

func SchollzCallerPAKEExchange(pw []byte, yakkMailBoxConnection *YakkMailBoxConnection) error {
	// Create a new PAKE object for the mailbox connection
	pakeObj, err := pake.Init(pw, 0, elliptic.P256(), 50*time.Millisecond)
	if err != nil {
		return err
	}
	// Send PAKE bytes over to the callee
	yakkMailBoxConnection.SendMsg(
		YAKKMSG_STARTPAKEEXCHANGE,
		base64.StdEncoding.EncodeToString(pakeObj.Bytes()),
	)
	yakkMailBoxConnection.State = YAKK_EXCHANGINGPAKE

HandleWSMsgLoop:
	for {
		mailBoxMsg := <-yakkMailBoxConnection.RecvChannel

		switch mailBoxMsg.Msg_type {
		case YAKKMSG_PAKEEXCHANGE:
			if yakkMailBoxConnection.State == YAKK_EXCHANGINGPAKE {
				P_bytes, err := base64.StdEncoding.DecodeString(mailBoxMsg.Payload)
				if err != nil {
					panic(err)
				}
				err = pakeObj.Update(P_bytes)
				if err != nil {
					panic(err)
				}
				yakkMailBoxConnection.SendMsg(
					YAKKMSG_PAKEEXCHANGE,
					base64.StdEncoding.EncodeToString(pakeObj.Bytes()),
				)
				yakkMailBoxConnection.State = YAKK_INITIALISED
				break HandleWSMsgLoop
			}
		}
	}
	log.Info().Msg("PAKE Exchanged Successfully")
	return nil
}

func SchollzCalleePAKEExchange(pw []byte, yakkMailBoxConnection *YakkMailBoxConnection) error {

HandleWSMsgLoop:
	for {
		mailBoxMsg := <-yakkMailBoxConnection.RecvChannel
		var pakeObj *pake.Pake
		switch mailBoxMsg.Msg_type {
		case YAKKMSG_STARTPAKEEXCHANGE:
			// This is the first message. Initialise state and sender/recipient IDs
			// Create a new PAKE object for the mailbox connection
			pakeObj, err := pake.Init(pw, 1, elliptic.P256(), 50*time.Millisecond)
			if err != nil {
				return err
			}
			yakkMailBoxConnection.SenderID = mailBoxMsg.Recipient
			yakkMailBoxConnection.RecipientID = mailBoxMsg.Sender
			yakkMailBoxConnection.State = YAKK_UNINITIALISED

			P_bytes, err := base64.StdEncoding.DecodeString(mailBoxMsg.Payload)
			if err != nil {
				panic(err)
			}
			err = pakeObj.Update(P_bytes)
			if err != nil {
				log.Debug().Msgf("Error: %s", err.Error())
				panic(err)
			}
			yakkMailBoxConnection.SendMsg(
				YAKKMSG_PAKEEXCHANGE,
				base64.StdEncoding.EncodeToString(pakeObj.Bytes()),
			)
			yakkMailBoxConnection.State = YAKK_EXCHANGINGPAKE
		case YAKKMSG_PAKEEXCHANGE:
			if yakkMailBoxConnection.State == YAKK_EXCHANGINGPAKE {
				P_bytes, err := base64.StdEncoding.DecodeString(mailBoxMsg.Payload)
				if err != nil {
					panic(err)
				}
				err = pakeObj.Update(P_bytes)
				if err != nil {
					panic(err)
				}
				yakkMailBoxConnection.SendMsg(
					YAKKMSG_PAKEEXCHANGE,
					base64.StdEncoding.EncodeToString(pakeObj.Bytes()),
				)
				log.Info().Msgf("Connection Key is verified: %d", pakeObj.IsVerified())
				yakkMailBoxConnection.State = YAKK_INITIALISED
				break HandleWSMsgLoop
			}
		}
	}
	log.Info().Msg("Completed PAKE Exchange")
	return nil
}
