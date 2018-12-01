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
	addrListenFlag     = flag.String("listen-addr", ":4200", "listening address of the server")
	httpAddrListenFlag = flag.String("http-listen-addr", ":4280", "listening address of the internal HTTP server")
	httpAssetsRootFlag = flag.String("http-assets-root", "/usr/share/cellaserv/http", "location of the http assets")
	httpExternalUrl    = flag.String("http-external-url", "", "prefix of the web component URLs")
)

func versionAndDie() {
	fmt.Println("cellaserv3 version", common.Version)
	os.Exit(0)
}

func locateHttpAssets() string {
	locations := []string{
		*httpAssetsRootFlag,
		"broker/web/ui",       // when started from repository root
		"../../broker/web/ui", // when started from the location of this file
	}
	exists := func(path string) bool {
		_, err := os.Stat(path)
		return !os.IsNotExist(err)
	}
	for _, location := range locations {
		if exists(location) {
			return location
		}
	}
	return *httpAssetsRootFlag
}

func main() {
	// Parse command line arguments
	flag.Parse()

	if *versionFlag {
		versionAndDie()
	}

	common.LogSetup()

	// Broker component
	brokerOptions := &broker.Options{
		ListenAddress: *addrListenFlag,
	}
	broker := broker.New(brokerOptions, logging.MustGetLogger("broker"))

	// Web component
	webOptions := &web.Options{
		ListenAddr:      *httpAddrListenFlag,
		AssetsPath:      locateHttpAssets(),
		ExternalURLPath: *httpExternalUrl,
	}
	webHander := web.New(webOptions, logging.MustGetLogger("web"), broker)

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
