package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"

	""github.com/choonkiatlee/yakk/yakk"
	"github.com/choonkiatlee/yakk/yakkutils""
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
	Host               string `long:"host" description:"Local IP to bind to. Defaults to 0.0.0.0" default:"127.0.0.1"`
	SignalingServerURL string `short:"s" long:"signalling-server-url" description:"URL for the signalling server" default:"ckl41.user.srcf.net"`
	KeepAlive          bool   `long:"keepalive" description:"whether or not to keep the server connection alive for a while longer after all connections have closed"`
}

var ClientOpts struct {
	// Slice of bool will append 'true' each time the option
	// is encountered (can be set multiple times, like -vvv)
	Verbose            []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
	Port               string `short:"p" long:"port" description:"TCP Port to listen" required:"True"`
	Host               string `long:"host" description:"Local IP to bind to. Defaults to 0.0.0.0" default:"127.0.0.1"`
	PW                 string `long:"pw" description:"Mail Room Password" default:""`
	SignalingServerURL string `short:"s" long:"signalling-server-url" description:"URL for the signalling server" default:"ckl41.user.srcf.net"`
}

var FileSendReceiveOpts struct {
	Verbose            []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
	Port               string `short:"p" long:"port" description:"TCP Port to listen. Defaults to a random open port" default:"-1"`
	PW                 string `long:"pw" description:"Mail Room Password" default:""`
	KeepAlive          bool   `long:"keepalive" description:"whether or not to keep the server connection alive for a while longer after all connections have closed"`
	SignalingServerURL string `short:"s" long:"signalling-server-url" description:"URL for the signalling server" default:"ckl41.user.srcf.net"`
}

// Define command line arguments
func parseServerArgs() {
	_, err := flags.Parse(&ServerOpts)

	if err != nil {
		panic(err)
	}
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

func parseFileSendReceiveArgs() (string, string) {
	args, err := flags.Parse(&FileSendReceiveOpts)
	yakk.PanicOnErr(err)

	port, err := strconv.Atoi(FileSendReceiveOpts.Port)
	yakk.PanicOnErr(err)
	if port < 0 {
		port, err = yakk.GetRandomOpenPort()
	}
	yakk.PanicOnErr(err)

	if len(args) > 1 {
		return args[1], fmt.Sprint(port)
	} else {
		return "", fmt.Sprint(port)
	}
}

func main() {

	cmd := ""

	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	default:
		fmt.Println(usage)
	case "client":
		// The client calls the server to start a connection
		roomID := parseClientArgs()
		InitLogBasedOnVerbosity(ClientOpts.Verbose)
		fmt.Println("Client Mode. Listening on: ", ClientOpts.Host, ClientOpts.Port)
		callerWaitGroup := &sync.WaitGroup{}
		peerConnection, _, err := yakk.Caller(roomID, []byte(ClientOpts.PW), callerWaitGroup, ClientOpts.SignalingServerURL)
		yakk.PanicOnErr(err)
		go yakk.ListenOnTCP(net.JoinHostPort(ClientOpts.Host, ClientOpts.Port), peerConnection)
		callerWaitGroup.Wait()

	case "server":
		// The server starts up and makes itself ready for a call
		// from the caller
		parseServerArgs()
		InitLogBasedOnVerbosity(ServerOpts.Verbose)
		fmt.Println("Server Mode. Connected to: ", ServerOpts.Host, ServerOpts.Port)
		// Create a mailbox first
		callerWaitGroup := &sync.WaitGroup{}
		_, err := yakk.Callee(callerWaitGroup, "client -p <port>", ServerOpts.KeepAlive, ServerOpts.SignalingServerURL)
		if err != nil {
			panic(err)
		}
		callerWaitGroup.Wait()
		log.Info().Msg("No more connections...Shutting down...")

	case "filesend":
		filename, port := parseFileSendReceiveArgs()
		InitLogBasedOnVerbosity(FileSendReceiveOpts.Verbose)
		fmt.Println("Serving File: " + filename)

		// Create a server connection on the specified port
		calleeWaitGroup := &sync.WaitGroup{}
		peerConnection, err := yakk.Callee(calleeWaitGroup, "filereceive", FileSendReceiveOpts.KeepAlive, FileSendReceiveOpts.SignalingServerURL)
		log.Info().Msg("connected to peer")
		yakk.PanicOnErr(err)
		yakk.ConnectToTCP(net.JoinHostPort("", port), peerConnection)
		log.Info().Msg("Connected to TCP")

		// Create a httpserver to serve files on that specified port
		httpServerExitDone := &sync.WaitGroup{}
		httpServerExitDone.Add(1)
		srv := yakkutils.ServeFile(filename, port, httpServerExitDone)
		log.Debug().Msg("Serving file on http server...")
		calleeWaitGroup.Wait()
		if err := srv.Shutdown(context.TODO()); err != nil {
			panic(err)
		}
		httpServerExitDone.Wait()

	case "filereceive":
		roomID, port := parseFileSendReceiveArgs()
		InitLogBasedOnVerbosity(FileSendReceiveOpts.Verbose)

		// Create a connection to the sender
		callerWaitGroup := &sync.WaitGroup{}
		peerConnection, _, err := yakk.Caller(roomID, []byte(FileSendReceiveOpts.PW), callerWaitGroup, FileSendReceiveOpts.SignalingServerURL)
		yakk.PanicOnErr(err)
		log.Debug().Msg("Connected through webrtc")
		go yakk.ListenOnTCP(net.JoinHostPort("", port), peerConnection)

		// pull the file down from the server
		yakkutils.ReceiveFile("", port)
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
