package broker

import (
	"testing"
	"time"

	"github.com/evolutek/cellaserv3/testutil"
)

func TestSubscribe(t *testing.T) {
	brokerTest(t, func(b *Broker) {
		conn := testutil.Dial(t)
		defer conn.Close()

		const topic = "test"
		conn.Write(testutil.MakeMessageSubscribe(t, topic))

		time.Sleep(50 * time.Millisecond)
	})
}
