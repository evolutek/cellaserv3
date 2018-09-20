package broker

import (
	"testing"

	"github.com/evolutek/cellaserv3/testutil"
)

func TestRequestNoService(t *testing.T) {
	go func() {
		conn := testutil.Dial(t)
		defer conn.Close()

		payload := []byte{}
		msg := testutil.MakeMessageRequest(t, "foo", "bar", "lol", payload)
		conn.Write(msg)

		// Cleanup
		handleShutdown()
	}()

	ListenAndServe(":4200")
}
