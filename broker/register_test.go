package broker

import (
	"testing"
	"time"

	"github.com/evolutek/cellaserv3/testutil"
)

func serviceIsRegistered(b *Broker, t *testing.T, serviceName string, serviceIdent string) {
	idents, found := b.Services[serviceName]
	if !found {
		t.Fail()
		return
	}
	if _, found := idents[serviceIdent]; !found {
		t.Fail()
		return
	}
}

func TestRegister(t *testing.T) {
	brokerTest(t, func(b *Broker) {
		conn := testutil.Dial(t)
		defer conn.Close()

		// Send register message
		const serviceName = "testName"
		const serviceIdent = "testIdent"
		msg := testutil.MakeMessageRegister(t, serviceName, serviceIdent)
		conn.Write(msg)

		time.Sleep(50 * time.Millisecond)

		// The service is registered
		serviceIsRegistered(b, t, serviceName, serviceIdent)
	})
}

func TestRegisterReplace(t *testing.T) {
	brokerTest(t, func(b *Broker) {
		conn := testutil.Dial(t)
		defer conn.Close()

		// Register first service
		const serviceName = "testName"
		const serviceIdent = "testIdent"
		registerMsg := testutil.MakeMessageRegister(t, serviceName, serviceIdent)
		conn.Write(registerMsg)

		// Register the service again
		conn.Write(registerMsg)

		time.Sleep(50 * time.Millisecond)

		// The new service has replaced the old one
		serviceIsRegistered(b, t, serviceName, serviceIdent)

		// Register the service again, with a different connection
		conn2 := testutil.Dial(t)
		defer conn2.Close()
		conn2.Write(registerMsg)

		time.Sleep(50 * time.Millisecond)

		// The new service has replaced the old one
		serviceIsRegistered(b, t, serviceName, serviceIdent)
	})
}
