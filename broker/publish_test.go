package broker

import (
	"testing"

	"github.com/evolutek/cellaserv3/testutil"
)

func TestPublishNoSubscriber(t *testing.T) {
	go func() {
		conn := testutil.Dial(t)
		defer conn.Close()

		const topic = "test"
		conn.Write(testutil.MakeMessagePublish(t, topic))

		handleShutdown()
	}()

	ListenAndServe(":4200")
}
