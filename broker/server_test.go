package broker

import (
	"os"
	"testing"

	"github.com/evolutek/cellaserv3/common"
)

func listenAndServeForTest(t *testing.T) {
	if err := ListenAndServe(":4200"); err != nil {
		t.Error(err)
	}
}

func TestMain(m *testing.M) {
	// Make sure the tests have logging setup
	common.LogSetup()

	// Start the tests
	os.Exit(m.Run())
}
