package main

import (
	"fmt"
	"net"
	"os"

	"github.com/choonkiatlee/yakk/yakk"
	"github.com/jessevdk/go-flags"
)

const usage = `Usage: ssh-p2p SUBCMD [options]
sub-commands:
	newkey
		new generate key of connection
	server -
		ssh server side peer mode
	client -key="..." [-listen="127.0.0.1:2222"]
		ssh client side peer mode
`

var ServerOpts struct {
	// Slice of bool will append 'true' each time the option
	// is encountered (can be set multiple times, like -vvv)
	Verbose            bool   `short:"v" long:"verbose" description:"Show verbose debug information"`
	Port               string `short:"p" long:"port" description:"TCP Port to listen" required:"True"`
	Host               string `long:"host" description:"Local IP to bind to. Defaults to 0.0.0.0" default:"127.0.0.1"`
	SignalingServerURL string `short:"s" long:"signalling-server-url" description:"URL for the signalling server" default:"https://127.0.0.1:6006"`
}

var ClientOpts struct {
	// Slice of bool will append 'true' each time the option
	// is encountered (can be set multiple times, like -vvv)
	Verbose            bool   `short:"v" long:"verbose" description:"Show verbose debug information"`
	Port               string `short:"p" long:"port" description:"TCP Port to listen" required:"True"`
	Host               string `long:"host" description:"Local IP to bind to. Defaults to 0.0.0.0" default:"127.0.0.1"`
	SignalingServerURL string `short:"s" long:"signalling-server-url" description:"URL for the signalling server" default:"https://127.0.0.1:6006"`
}

// Define command line arguments
func parseServerArgs() {
	_, err := flags.Parse(&ServerOpts)

	if err != nil {
		panic(err)
	}
}

func parseClientArgs() []string {
	args, err := flags.Parse(&ClientOpts)

	if err != nil {
		panic(err)
	}
	return args
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
		args := parseClientArgs()
		fmt.Println("Client Mode...")
		if len(args) > 1 {
			yakk.Caller(args[1], net.JoinHostPort(ClientOpts.Host, ClientOpts.Port))
		} else {
			yakk.Caller("", net.JoinHostPort(ClientOpts.Host, ClientOpts.Port))
		}
	case "server":
		// The server starts up and makes itself ready for a call
		// from the caller
		fmt.Println("Server Mode...")
		parseServerArgs()
		fmt.Println(ServerOpts.Host, ServerOpts.Port)
		// Create a mailbox first
		yakk.Callee(net.JoinHostPort(ServerOpts.Host, ServerOpts.Port))
	}
}
