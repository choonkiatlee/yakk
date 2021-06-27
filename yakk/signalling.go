package yakk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"cloud.google.com/go/firestore"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
)

func FirestoreAnonymousSignIn() string {
	url := "https://identitytoolkit.googleapis.com/v1/accounts:signUp?key=AIzaSyB2VsP-7dQNTuxISq-liuFOly4CYhOVEhM"
	payload := []byte(`{"returnSecureToken": true}`)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	PanicOnErr(err)
	body, err := ioutil.ReadAll(resp.Body)
	PanicOnErr(err)

	var data map[string]string
	err = json.Unmarshal(body, &data)
	PanicOnErr(err)

	token := data["idToken"]
	log.Info().Msgf("Firebase Token: %s", token)
	return token
}

func InitFirestoreConnection(firebaseProject string, jwtToken string) (*firestore.Client, error) {
	ctx := context.Background()
	tokenProvider := tokenProvider{
		token: jwtToken,
	}
	client, err := firestore.NewClient(
		ctx,
		firebaseProject,
		option.WithTokenSource(tokenProvider),
	)
	return client, err
}

func InitMailBoxConnection(firebaseProject string, jwtToken string) (*YakkMailBoxConnection, error) {
	client, err := InitFirestoreConnection(firebaseProject, jwtToken)
	if err != nil {
		return &YakkMailBoxConnection{}, err
	}

	conn := YakkMailBoxConnection{
		firebaseClient: client,
		State:          YAKKSTATE_UNINITIALISED,
		RecvChan:       make(chan YakkMailBoxMsg),
	}
	return &conn, nil
}

func (yakkMailBoxConnection *YakkMailBoxConnection) getCollectionRef() *firestore.CollectionRef {
	collection := yakkMailBoxConnection.firebaseClient.Collection("mailrooms")
	return collection
}

func (yakkMailBoxConnection *YakkMailBoxConnection) getMailRoomDocRef() *firestore.DocumentRef {
	collection := yakkMailBoxConnection.getCollectionRef()
	docRef := collection.Doc(yakkMailBoxConnection.RoomName)
	return docRef
}

func (yakkMailBoxConnection *YakkMailBoxConnection) getMailBoxDocRef() *firestore.DocumentRef {
	collection := yakkMailBoxConnection.getMailRoomDocRef().Collection("MailBoxes")
	docRef := collection.Doc(yakkMailBoxConnection.MailBoxName)
	return docRef
}

func (yakkMailBoxConnection *YakkMailBoxConnection) getMailBoxMessagesRef(msgToHeader string) *firestore.DocumentRef {
	collection := yakkMailBoxConnection.getMailBoxDocRef().Collection("Msgs")
	docRef := collection.Doc(msgToHeader)
	return docRef
}

func (yakkMailBoxConnection *YakkMailBoxConnection) SendMsg(msgType string, payload string) error {
	var msgTo string
	if yakkMailBoxConnection.isOwner {
		msgTo = "OtoRMsg"
	} else {
		msgTo = "RtoOMsg"
	}
	docRef := yakkMailBoxConnection.getMailBoxMessagesRef(msgTo)
	updates := []firestore.Update{
		{
			Path:  "MsgType",
			Value: msgType,
		},
		{
			Path:  "Payload",
			Value: payload,
		},
	}
	_, err := docRef.Update(
		context.Background(), updates,
	)

	return err
}

func (yakkMailBoxConnection *YakkMailBoxConnection) ListenForMsgs() {
	var msgTo string
	if yakkMailBoxConnection.isOwner {
		msgTo = "RtoOMsg"
	} else {
		msgTo = "OtoRMsg"
	}
	docRef := yakkMailBoxConnection.getMailBoxMessagesRef(msgTo)
	snapIter := docRef.Snapshots(context.Background())
	defer snapIter.Stop()

	for {
		snap, err := snapIter.Next()
		if err != nil {
			fmt.Printf("Error in listening for msg. %s", err)
		}
		var msg YakkMailBoxMsg
		snap.DataTo(&msg)
		if msg.MsgType != "" {
			yakkMailBoxConnection.RecvChan <- msg
		}
	}
}

func InitMailRoomConnection(firebaseProject string, jwtToken string) (*YakkMailRoomConnection, error) {
	client, err := InitFirestoreConnection(firebaseProject, jwtToken)
	if err != nil {
		return &YakkMailRoomConnection{}, err
	}

	conn := YakkMailRoomConnection{
		firebaseClient: client,
		MailBoxes:      make(map[string]*YakkMailBoxConnection),
	}
	return &conn, nil
}

func (yakkMailRoomConnection *YakkMailRoomConnection) getMailRoomCollectionRef() *firestore.CollectionRef {
	collection := yakkMailRoomConnection.firebaseClient.Collection("mailrooms")
	return collection
}

func (yakkMailRoomConnection *YakkMailRoomConnection) getMailBoxCollectionRef() *firestore.CollectionRef {
	collection := yakkMailRoomConnection.firebaseClient.Collection(
		"mailrooms/" + yakkMailRoomConnection.RoomName + "/MailBoxes",
	)
	return collection
}
