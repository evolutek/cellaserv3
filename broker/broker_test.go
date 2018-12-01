package broker

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/evolutek/cellaserv3/common"
	logging "gopkg.in/op/go-logging.v1"
)

// brokerTest is a test harness for testing the broker. It takes care of
// setting up the server, and shutting it down when testing is over.
func brokerTest(t *testing.T, testFn func(b *Broker)) {
	ctxBroker, cancelBroker := context.WithCancel(context.Background())
	brokerOptions := &Options{ListenAddress: ":4200"}
	broker := New(brokerOptions, logging.MustGetLogger("broker"))

	go func() {
		err := broker.Run(ctxBroker)
		if err != nil {
			t.Fatalf("Could not start broker: %s", err)
		}
	}()

	// Give time to the broker to start
	time.Sleep(50 * time.Millisecond)

	// Run the test
	testFn(broker)
	time.Sleep(50 * time.Millisecond)

	// Teardown broker
	cancelBroker()
	time.Sleep(50 * time.Millisecond)
}

// TestMain is called at the begining of the test suite.
func TestMain(m *testing.M) {
	// Make sure the tests have logging setup
	common.LogSetup()

	// Start the tests
	os.Exit(m.Run())
}
