package main

import (
	"time"

	cellaserv "github.com/evolutek/cellaserv3-protobuf"
	"github.com/evolutek/cellaserv3/client"
)

func runDateService(opts client.ClientOpts) {
	// Connect to cellaserv
	conn := client.NewClient(opts)

	// Prepare service for registration
	date := conn.NewService("date", "")
	// Handle "time" request
	date.HandleRequestFunc("time", func(_ *cellaserv.Request) (interface{}, error) {
		return time.Now(), nil
	})
	// Handle "killall" event
	date.HandleEventFunc("killall", func(_ *cellaserv.Publish) {
		conn.Close()
	})

	// Register the service on cellaserv
	conn.RegisterService(date)

	<-conn.Quit()
}

func main() {
	runDateService(client.ClientOpts{})
}
