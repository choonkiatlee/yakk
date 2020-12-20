package main

import (
	"fmt"
	"net/http"

	"github.com/choonkiatlee/yakk/yakkserver"
	"github.com/jessevdk/go-flags"
)

const usage = `Usage: yakkserver [options]
sub-commands:
	--port = TCP Port to listen to
	--ip  = Local IP to bind to
`

var SignallingServerOpts struct {
	// Slice of bool will append 'true' each time the option
	// is encountered (can be set multiple times, like -vvv)
	Verbose bool `short:"v" long:"verbose" description:"Show verbose debug information"`
	// LocalListenPort string `long:"port" description:"TCP Port to listen" required:"True"`
	LocalListenPort int    `long:"port" description:"TCP Port to listen" default:"6006"`
	LocalIP         string `long:"ip" description:"Local IP to bind to. Defaults to 127.0.0.1" default:"127.0.0.1"`
}

func parseSignallingServerArgs() {
	_, err := flags.Parse(&SignallingServerOpts)

	if err != nil {
		panic(err)
	}
}

func main() {
	parseSignallingServerArgs()
	fmt.Println("Running server")
	fmt.Println(SignallingServerOpts.LocalIP, SignallingServerOpts.LocalListenPort)

	addr := fmt.Sprintf("%s:%d", SignallingServerOpts.LocalIP, SignallingServerOpts.LocalListenPort)

	http.HandleFunc("/ws/", yakkserver.HandleMessage)
	http.HandleFunc("/", yakkserver.Hello)
	go yakkserver.CleanMailBoxes()
	http.ListenAndServe(addr, nil)
}
