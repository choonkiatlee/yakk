package yakk

import (
	"context"
	"fmt"
	"log"

	"cloud.google.com/go/firestore"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

type tokenProvider struct {
	token string
}

func (t tokenProvider) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: t.token,
	}, nil
}

func createFirestoreClient() (*firestore.Client, context.Context) {
	ctx := context.Background()

	tokenProvider := tokenProvider{
		token: "eyJhbGciOiJSUzI1NiIsImtpZCI6ImRjNGQwMGJjM2NiZWE4YjU0NTMzMWQxZjFjOTZmZDRlNjdjNTFlODkiLCJ0eXAiOiJKV1QifQ.eyJwcm92aWRlcl9pZCI6ImFub255bW91cyIsImlzcyI6Imh0dHBzOi8vc2VjdXJldG9rZW4uZ29vZ2xlLmNvbS95YWtrLTUwYWU4IiwiYXVkIjoieWFray01MGFlOCIsImF1dGhfdGltZSI6MTYyNDIxODI5MiwidXNlcl9pZCI6IlF6STVqY0xRblRiWVpQaGJLTjU1amIzM2tUWDIiLCJzdWIiOiJRekk1amNMUW5UYllaUGhiS041NWpiMzNrVFgyIiwiaWF0IjoxNjI0MjE4MjkyLCJleHAiOjE2MjQyMjE4OTIsImZpcmViYXNlIjp7ImlkZW50aXRpZXMiOnt9LCJzaWduX2luX3Byb3ZpZGVyIjoiYW5vbnltb3VzIn19.gUyQrQHKhQfW9ATVNGSLv-WiWU-kdJBA8vgtXzByu4znnLLzmgMIJz43mpc_Ffh0yBe15MD484lOLzyBtrG2W2qbxqPfYEXKX_VWgjBPDrs9nW-oLi2VV76Og5lf3f42qYZuJsFiYXNUlB7BdFiIu-74nxBl4zWjzUKs_uR1f-MP3gWstqyPhgP_hV9-R4aBVBYzL6dbOV0iBCKjB7Zk2Ad55Bz28hZ4U-7M1DNXYgPJakVwFOBmi53PiMluH-fqAJuzLoqrahQ4GhDEzoUp1X6wVT0qeK5bH0AQTZ_3DMn8KjrZK6SQr_frJUJIs9ARn3pCKf8MsYxbiEXYhBJelg",
	}
	fmt.Println("Connecting to client...")
	client, err := firestore.NewClient(
		ctx,
		"yakk-50ae8",
		option.WithTokenSource(tokenProvider),
	)
	if err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}
	return client, ctx
}

func Test() {

	client, ctx := createFirestoreClient()
	defer client.Close() // Close client when done.

	fmt.Println("Connected to client")
	colref := client.Collection("mailboxes")
	fmt.Print(colref)
	ny := colref.Doc("NewYork")
	fmt.Print(ny)
	// Or, in a single call:
	_, err := ny.Create(ctx, map[string]interface{}{
		"id":    "user1",
		"name":  "username",
		"state": 0,
	})
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Set Document")
	client.Close()
}
