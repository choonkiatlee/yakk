package main

import (
	"fmt"
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
	LocalListenPort    string `short:"l" long:"listen-port" description:"TCP Port to listen" required:"True"`
	LocalIP            string `long:"ip" description:"Local IP to bind to. Defaults to 0.0.0.0" default:"127.0.0.1"`
	SignalingServerURL string `short:"s" long:"signalling-server-url" description:"URL for the signalling server" default:"https://127.0.0.1:6006"`
}

var ClientOpts struct {
	// Slice of bool will append 'true' each time the option
	// is encountered (can be set multiple times, like -vvv)
	Verbose            bool   `short:"v" long:"verbose" description:"Show verbose debug information"`
	LocalListenPort    string `short:"l" long:"listen-port" description:"TCP Port to listen" required:"True"`
	LocalIP            string `long:"ip" description:"Local IP to bind to. Defaults to 0.0.0.0" default:"127.0.0.1"`
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
		args := parseClientArgs()
		fmt.Println("Client Mode...")
		if len(args) > 1 {
			yakk.Callee(args[1])
		} else {
			yakk.Callee("")
		}
	case "server":
		fmt.Println("Server Mode...")
		parseServerArgs()
		fmt.Println(ServerOpts.LocalIP, ServerOpts.LocalListenPort)
		// Create a mailbox first
		yakk.Caller()
	}
}
