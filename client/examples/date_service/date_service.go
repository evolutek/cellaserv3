package main

import (
	"time"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"bitbucket.org/evolutek/cellaserv3/client"
)

func main() {
	// Connect to cellaserv
	conn := client.NewClient(client.ClientOpts{})

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
