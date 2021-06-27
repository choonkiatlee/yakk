package yakk

import (
	"encoding/base64"
	"errors"

	"github.com/choonkiatlee/jpake-go"
	"github.com/rs/zerolog/log"
)

func JPAKEExchange(pw []byte, yakkMailBoxConnection *YakkMailBoxConnection, checkSessionKey bool) error {
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
	if err != nil {
		return err
	}
	log.Debug().Msg("Sent First Round Msg")

HandleWSMsgLoop:
	for {
		mailBoxMsg := <-yakkMailBoxConnection.RecvChan
		log.Debug().Msgf("Received Msg of type: %s", mailBoxMsg.MsgType)
		switch mailBoxMsg.MsgType {
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
			yakkMailBoxConnection.State = YAKKSTATE_EXCHANGINGPAKE
		case YAKKMSG_JPAKEROUND2:
			if yakkMailBoxConnection.State == YAKKSTATE_EXCHANGINGPAKE {
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
					yakkMailBoxConnection.State = YAKKSTATE_EXCHANGINGPAKE2
				} else {
					yakkMailBoxConnection.SessionKey = aliceSharedKey
					yakkMailBoxConnection.State = YAKKSTATE_INITIALISED
					break HandleWSMsgLoop
				}
			}
		case YAKKMSG_JPAKEKEYCONFIRMATION:
			if yakkMailBoxConnection.State == YAKKSTATE_EXCHANGINGPAKE2 {
				bobCheckSessionKeyMsg, err := base64.StdEncoding.DecodeString(mailBoxMsg.Payload)
				if err != nil {
					return err
				}
				verified := pakeObj.CheckReceivedSessionKeyMsg([]byte(bobCheckSessionKeyMsg))
				if verified {
					yakkMailBoxConnection.SessionKey, err = pakeObj.SessionKey()
					yakkMailBoxConnection.State = YAKKSTATE_INITIALISED
					if err != nil {
						return err
					}
					break HandleWSMsgLoop
				} else {
					return errors.New("could not verify the session key")
				}
			}
		}
	}
	log.Info().Msg("Completed PAKE Exchange")
	return nil
}
