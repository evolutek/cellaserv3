// The cellaserv server entry point.
//
// Defines command line flags, loads configuration, and start the server.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/evolutek/cellaserv3/broker"
	"github.com/evolutek/cellaserv3/broker/web"
	"github.com/evolutek/cellaserv3/common"
	"github.com/oklog/run"
	"github.com/prometheus/common/log"

	logging "gopkg.in/op/go-logging.v1"
)

var (
	// Command line flags
	versionFlag        = flag.Bool("version", false, "output version information and exit")
	sockAddrListenFlag = flag.String("listen-addr", ":4200", "listening address of the server")
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

	brokerOptions := &broker.Options{
		ListenAddress: *sockAddrListenFlag,
	}
	broker := broker.New(logging.MustGetLogger("broker"), brokerOptions)

	webHander := web.New(logging.MustGetLogger("web"), broker)

	// Contexts
	ctxBroker, cancelBroker := context.WithCancel(context.Background())
	ctxWeb, cancelWeb := context.WithCancel(context.Background())

	// Setup goroutines
	var g run.Group
	{
		//  Broker
		g.Add(func() error {
			if err := broker.Run(ctxBroker); err != nil {
				return fmt.Errorf("error starting the broker: %s", err)
			}
			return nil
		}, func(error) {
			cancelBroker()
		})

		// Web handler
		g.Add(func() error {
			if err := webHander.Run(ctxWeb); err != nil {
				return fmt.Errorf("error starting the broker: %s", err)
			}

			return nil
		}, func(error) {
			cancelWeb()
		})
	}

	if err := g.Run(); err != nil {
		log.Errorf("Error: %s", err)
		os.Exit(1)
	}
}
