// The cellaserv server entry point.
//
// Defines command line flags, loads configuration, and start the server.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"bitbucket.org/evolutek/cellaserv3/broker"
	"bitbucket.org/evolutek/cellaserv3/broker/cellaserv"
	"bitbucket.org/evolutek/cellaserv3/broker/web"
	"bitbucket.org/evolutek/cellaserv3/common"

	"github.com/oklog/run"
	"github.com/pkg/errors"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

func locateHttpAssets(userLocation string) string {
	locations := []string{
		userLocation,
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
	return userLocation
}

func main() {
	brokerOptions := broker.Options{}
	webOptions := web.Options{}

	a := kingpin.New(filepath.Base(os.Args[0]), "The cellaserv message broker")
	a.Version(common.GetVersion())
	a.HelpFlag.Short('h')

	// Broker options
	a.Flag("listen-addr", "listening address of the server").
		Default(":4200").
		StringVar(&brokerOptions.ListenAddress)

	// Publish logging
	a.Flag("store-logs", "whether to store logs, enables using cellaserv.get_logs()").
		Default("true").
		BoolVar(&brokerOptions.PublishLoggingEnabled)
	a.Flag("logs-dir", "base path for client logs storage").
		Default("/var/log/cellaserv").
		StringVar(&brokerOptions.LogsDir)

	// Web options
	a.Flag("http-listen-addr", "listening address of the internal HTTP server").
		Default(":4280").
		StringVar(&webOptions.ListenAddr)
	a.Flag("http-assets-root", "location of the http assets").
		Default("/usr/share/cellaserv/http").
		StringVar(&webOptions.AssetsPath)
	a.Flag("http-external-url", "prefix of the web component URLs").
		StringVar(&webOptions.ExternalURLPath)

	common.AddFlags(a)

	_, err := a.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Could not parse command line arguments"))
		a.Usage(os.Args[1:])
		os.Exit(2)
	}

	webOptions.AssetsPath = locateHttpAssets(webOptions.AssetsPath)

	log := common.NewLogger("cellaserv")

	// Broker component
	broker := broker.New(brokerOptions, common.NewLogger("broker"))

	// Cellaserv service
	csOpts := &cellaserv.Options{BrokerAddr: brokerOptions.ListenAddress}
	cs := cellaserv.New(csOpts, broker, common.NewLogger("cellaserv"))

	// Web component
	webHander := web.New(&webOptions, common.NewLogger("web"), broker)

	// Contexts
	ctxBroker, cancelBroker := context.WithCancel(context.Background())
	ctxWeb, cancelWeb := context.WithCancel(context.Background())

	// Setup goroutines
	var g run.Group
	{
		// Broker
		g.Add(func() error {
			if err := broker.Run(ctxBroker); err != nil {
				return fmt.Errorf("[Broker] Could not start: %s", err)
			}
			return nil
		}, func(error) {
			cancelBroker()
		})
	}
	{
		// Cellaserv service
		g.Add(func() error {
			if err := cs.Run(ctxBroker); err != nil {
				return fmt.Errorf("[Cellaserv] Could not start: %s", err)
			}
			return nil
		}, func(error) {
			cancelBroker()
		})
	}
	{
		// Web handler
		g.Add(func() error {
			if err := webHander.Run(ctxWeb); err != nil {
				return fmt.Errorf("[Web] Could not start web interface: %s", err)
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
