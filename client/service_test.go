package client

import (
	"context"
	"testing"
	"time"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/evolutek/cellaserv3/broker"
	logging "gopkg.in/op/go-logging.v1"
)

func TestServiceRequest(t *testing.T) {
	ctxBroker, cancelBroker := context.WithCancel(context.Background())

	brokerOptions := &broker.Options{
		ListenAddress: ":4201",
	}
	broker := broker.New(brokerOptions, logging.MustGetLogger("test"))

	go func() {
		err := broker.Run(ctxBroker)
		if err != nil {
			t.Fatalf("Could not start broker: %s", err)
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// Open connection
	connService := NewConnection(":4201")
	// Prepare service for registration
	dateService := connService.NewService("date", "")
	// Handle "time" request
	dateService.HandleRequestFunc("time", func(_ *cellaserv.Request) (interface{}, error) {
		return time.Now(), nil
	})
	// Register the service
	connService.RegisterService(dateService)

	time.Sleep(50 * time.Millisecond)

	// Create service client connection
	connRequest := NewConnection(":4201")
	dateServiceStub := NewServiceStub(connRequest, "date", "")

	// Test valid method
	dateServiceStub.Request("time", nil)

	// Testt invalid method
	_, err := dateServiceStub.Request("foobarlol", nil)
	if err == nil {
		t.Errorf("Did not return error on non-existing method")
	}

	// Shutdown cellaserv
	cancelBroker()
}
