package main

import (
	"time"

	"github.com/evolutek/cellaserv3/client"
	"github.com/evolutek/cellaserv3/common"
)

var log = common.GetLog()

func main() {
	// Connect to cellaserv
	conn := client.NewClient(client.ClientOpts{})

	// Publish the date event every second
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		conn.Publish("date", time.Now())
	}
}
