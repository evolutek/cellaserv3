package broker

import (
	"testing"
	"time"

	"github.com/evolutek/cellaserv3/testutil"
)

func TestPublishNoSubscriber(t *testing.T) {
	go func() {
		defer handleShutdown()

		conn := testutil.Dial(t)
		defer conn.Close()

		const topic = "test"
		conn.Write(testutil.MakeMessagePublish(t, topic))

		time.Sleep(50 * time.Millisecond)
	}()

	listenAndServeForTest(t)
}
