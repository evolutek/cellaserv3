package broker

import (
	"os"
	"testing"

	"github.com/evolutek/cellaserv3/common"
)

// listenAndServeForTest starts a broker on a predefined port
func listenAndServeForTest(t *testing.T) {
	if err := ListenAndServe(":4200"); err != nil {
		t.Error(err)
	}
}

// brokerTest is a test harness for testing the broker. It takes care of
// setting up the server, and shutting it down when testing is over.
func brokerTest(t *testing.T, testFn func()) {
	go func() {
		defer handleShutdown()
		testFn()
	}()

	listenAndServeForTest(t)
}

// TestMain is called at the begining of the test suite.
func TestMain(m *testing.M) {
	// Make sure the tests have logging setup
	common.LogSetup()

	// Start the tests
	os.Exit(m.Run())
}
