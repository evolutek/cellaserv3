package broker

import (
	"os"
	"testing"

	"github.com/evolutek/cellaserv3/common"
)

func TestMain(m *testing.M) {
	common.LogSetup()

	// Start the tests
	os.Exit(m.Run())
}
