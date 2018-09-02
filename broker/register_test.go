package broker

import (
	"net"
	"testing"
	"time"

	"github.com/evolutek/cellaserv3/testutil"
)

func TestRegister(t *testing.T) {
	go func() {
		conn, err := net.Dial("tcp", ":4200")
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		// Send register message
		const serviceName = "test"
		conn.Write(testutil.MakeMessageRegister(t, serviceName))

		time.Sleep(50 * time.Millisecond)

		// The service is added
		if _, ok := services[serviceName]; !ok {
			t.Fail()
		}

		// Kill the server
		handleShutdown()

		// Give it time to perform it's shutdown
		time.Sleep(50 * time.Millisecond)
	}()

	Serve()
}

func TestRegisterReplace(t *testing.T) {
	go func() {
		conn, err := net.Dial("tcp", ":4200")
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		// Register first service
		const serviceName = "test"
		conn.Write(testutil.MakeMessageRegister(t, serviceName))

		// Register second service
		conn.Write(testutil.MakeMessageRegister(t, serviceName))

		time.Sleep(50 * time.Millisecond)

		// The service is added
		if _, ok := services[serviceName]; !ok {
			t.Fail()
		}

		// Kill the server
		handleShutdown()

		// Give it time to perform it's shutdown
		time.Sleep(50 * time.Millisecond)
	}()

	Serve()
}
