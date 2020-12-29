package main

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/choonkiatlee/yakk/yakkserver"
	"github.com/jessevdk/go-flags"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const usage = `Usage: yakkserver [options]
sub-commands:
	--port = TCP Port to listen to
	--ip  = Local IP to bind to
`

var SignallingServerOpts struct {
	// Slice of bool will append 'true' each time the option
	// is encountered (can be set multiple times, like -vvv)
	Verbose            []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
	LocalListenPort    int    `long:"port" description:"TCP Port to listen" default:"6006"`
	LocalIP            string `long:"ip" description:"Local IP to bind to. Defaults to 0.0.0.0" default:"0.0.0.0"`
	UnixSocketFilename string `long:"unixsockfile" description:"Name of unix socket to bind to. This overrides ip / port" default:""`
}

func parseSignallingServerArgs() {
	_, err := flags.Parse(&SignallingServerOpts)

	if err != nil {
		panic(err)
	}
}

func main() {
	parseSignallingServerArgs()
	InitLogBasedOnVerbosity(SignallingServerOpts.Verbose)
	fmt.Println("Running server")
	fmt.Println(SignallingServerOpts.LocalIP, SignallingServerOpts.LocalListenPort)

	addr := fmt.Sprintf("%s:%d", SignallingServerOpts.LocalIP, SignallingServerOpts.LocalListenPort)

	http.HandleFunc("/ws/", yakkserver.HandleMessage)
	// http.HandleFunc("/", yakkserver.HandleMessage)
	http.HandleFunc("/hello", yakkserver.Hello)
	// go yakkserver.CleanMailBoxes()

	go yakkserver.CleanMailRooms()

	if len(SignallingServerOpts.UnixSocketFilename) > 0 {
		server := http.Server{
			Handler: http.DefaultServeMux,
		}

		unixListener, err := net.Listen("unix", SignallingServerOpts.UnixSocketFilename)
		if err != nil {
			panic(err)
		}

		server.Serve(unixListener)
	} else {
		http.ListenAndServe(addr, nil)
	}
}

func InitLogBasedOnVerbosity(verbosity []bool) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	switch len(verbosity) {
	case 0:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case 1:
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case 2:
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case 3:
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}
}
