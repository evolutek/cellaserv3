package main

import (
	"testing"
	"time"

	"github.com/evolutek/cellaserv3/client"
	"github.com/evolutek/cellaserv3/testutil/broker"
)

func TestDateService(t *testing.T) {
	broker.WithTestBroker(t, ":4202", func(clientOpts client.ClientOpts) {
		go runDateService(clientOpts)

		// Wait for the service to register
		time.Sleep(50 * time.Millisecond)

		// Create date service stub
		conn := client.NewClient(clientOpts)
		date := client.NewServiceStub(conn, "date", "")
		// Request date.time()
		_, err := date.Request("time", nil)

		if err != nil {
			t.Fatalf("Could not query date.time: %s", err)
		}

	})
}
