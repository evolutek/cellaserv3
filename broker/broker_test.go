package broker

import (
	"context"
	"testing"
	"time"

	"bitbucket.org/evolutek/cellaserv3/common"
)

// brokerTest is a test harness for testing the broker. It takes care of
// setting up the server, and shutting it down when testing is over.
func brokerTest(t *testing.T, testFn func(b *Broker)) {
	t.Helper()
	options := Options{ListenAddress: ":4200"}
	brokerTestWithOptions(t, options, testFn)
}

func brokerTestWithOptions(t *testing.T, options Options, testFn func(b *Broker)) {
	t.Helper()
	if options.ListenAddress == "" {
		options.ListenAddress = ":4200"
	}
	ctxBroker, cancelBroker := context.WithCancel(context.Background())
	broker := New(options, common.NewLogger("broker"))

	go func() {
		t.Helper()
		err := broker.Run(ctxBroker)
		if err != nil {
			t.Fatalf("Could not start broker: %s", err)
		}
	}()

	<-broker.startedCh

	// Run the test
	testFn(broker)
	time.Sleep(50 * time.Millisecond)

	// Teardown broker
	cancelBroker()
	time.Sleep(50 * time.Millisecond)
}
