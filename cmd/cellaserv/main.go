// The cellaserv server entry point.
//
// Defines command line flags, loads configuration, and start the server.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/evolutek/cellaserv3/broker"
	"github.com/evolutek/cellaserv3/common"
)

var (
	// Command line flags
	versionFlag = flag.Bool("version", false, "output version information and exit")
)

func versionAndDie() {
	fmt.Println("cellaserv3 version", common.Version)
	os.Exit(0)
}

func main() {
	// Parse command line arguments
	flag.Parse()

	if *versionFlag {
		versionAndDie()
	}

	common.LogSetup()

	broker.Serve()
}
