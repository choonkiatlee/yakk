package main

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/choonkiatlee/yakk/yakk"
	"github.com/jessevdk/go-flags"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const usage = `Usage: yakk SUBCMD [options]
sub-commands:
	server -
		ssh server side mode
	client -key="..." [-listen="127.0.0.1:2222"]
		ssh client side mode
`

var ServerOpts struct {
	// Slice of bool will append 'true' each time the option
	// is encountered (can be set multiple times, like -vvv)
	Verbose            []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
	Port               string `short:"p" long:"port" description:"TCP Port to listen" required:"True"`
	Host               string `long:"host" description:"Local IP to bind to. Defaults to 0.0.0.0" default:"0.0.0.0"`
	SignalingServerURL string `short:"s" long:"signalling-server-url" description:"URL for the signalling server" default:"yakk.herokuapp.com"`
	PW                 string `long:"pw" description:"Mail Room Password" default:""`
	KeepAlive          bool   `long:"keepalive" description:"whether or not to keep the server connection alive for a while longer after all connections have closed"`
}

var ClientOpts struct {
	// Slice of bool will append 'true' each time the option
	// is encountered (can be set multiple times, like -vvv)
	Verbose            []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
	Port               string `short:"p" long:"port" description:"TCP Port to listen" required:"True" default:"8000"`
	Host               string `long:"host" description:"Local IP to bind to. Defaults to 0.0.0.0" default:"0.0.0.0"`
	PW                 string `long:"pw" description:"Mail Room Password" default:""`
	SignalingServerURL string `short:"s" long:"signalling-server-url" description:"URL for the signalling server" default:"yakk.herokuapp.com"`
}

func parseClientArgs() string {
	args, err := flags.Parse(&ClientOpts)
	yakk.PanicOnErr(err)
	if len(args) > 1 {
		return args[1]
	} else {
		return ""
	}
}

func parseServerArgs() {
	_, err := flags.Parse(&ServerOpts)
	yakk.PanicOnErr(err)
}

func main() {

	firebaseProject := "yakk-50ae8"
	cmd := ""
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	default:
		fmt.Println(usage)
	case "client":
		fmt.Println("Client Mode.")
		roomID := parseClientArgs()
		pw := ClientOpts.PW
		if len(roomID) == 0 {
			roomID = yakk.GetInputFromStdin("Input MailRoom Name: ")
		}
		connectedSuccessString := "Connected. Listening on: " + ClientOpts.Host + ":" + ClientOpts.Port
		InitLogBasedOnVerbosity(ClientOpts.Verbose)
		token := yakk.FirestoreAnonymousSignIn()

		yakkMailBoxConnection, err := yakk.InitMailBoxConnection(firebaseProject, token)
		yakk.PanicOnErr(err)

		callerWaitGroup := &sync.WaitGroup{}
		peerConnection, _, err := yakk.Caller(yakkMailBoxConnection, roomID, []byte(pw), connectedSuccessString, callerWaitGroup)
		yakk.PanicOnErr(err)
		go yakk.ListenOnTCP(net.JoinHostPort(ClientOpts.Host, ClientOpts.Port), peerConnection)
		callerWaitGroup.Wait()
	case "server":
		fmt.Println("Server Mode.")

		parseServerArgs()
		pw := ServerOpts.PW
		connectedSuccessString := "Connected. Listening to: " + ServerOpts.Host + ":" + ServerOpts.Port
		InitLogBasedOnVerbosity(ServerOpts.Verbose)
		token := yakk.FirestoreAnonymousSignIn()

		yakkMailRoomConnection, err := yakk.InitMailRoomConnection(firebaseProject, token)
		yakk.PanicOnErr(err)

		roomName, err := yakk.CreateMailRoom(yakkMailRoomConnection)
		yakk.PanicOnErr(err)
		fmt.Printf("Your mailroom name is: %s\n", roomName)
		if len(pw) == 0 {
			pw = yakk.GetInputFromStdin("Input MailRoom PW: ")
		}
		fmt.Printf("One line connection command: yakk client %s --pw %s -p <port>\n", roomName, string(pw))

		calleeWaitGroup := &sync.WaitGroup{}
		err = yakk.Callee(yakkMailRoomConnection, []byte(pw), ServerOpts.Host, ServerOpts.Port, connectedSuccessString, calleeWaitGroup)
		yakk.PanicOnErr(err)
		calleeWaitGroup.Wait()
	}
}

func InitLogBasedOnVerbosity(verbosity []bool) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	switch len(verbosity) {
	case 0:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case 1:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case 2:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case 3:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}
}
