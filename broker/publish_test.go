package broker

import (
	"net"
	"testing"
	"time"

	"github.com/evolutek/cellaserv3/testutil"
)

func TestPublishNoSubscriber(t *testing.T) {
	go func() {
		conn, err := net.Dial("tcp", ":4200")
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		const topic = "test"
		conn.Write(testutil.MakeMessagePublish(t, topic))

		// Kill the server
		handleShutdown()

		// Give it time to perform it's shutdown
		time.Sleep(50 * time.Millisecond)
	}()

	Serve()
}
