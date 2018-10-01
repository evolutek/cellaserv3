package client

import (
	"testing"
	"time"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/evolutek/cellaserv3/broker"
)

func serviceRequestNoSuchMethod(t *testing.T, s *serviceStub) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Did not panic on invalid method")
		}
	}()

	s.Request("foobarlol", nil)
}

func TestServiceRequest(t *testing.T) {
	go func() {
		// Open connection
		connService := NewConnection(":4200")
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
		connRequest := NewConnection(":4200")
		dateServiceStub := connRequest.NewServiceStub("date", "")

		// Test valid method
		dateServiceStub.Request("time", nil)

		// Testt invalid method
		serviceRequestNoSuchMethod(t, dateServiceStub)

		// Shutdown cellaserv
		broker.Shutdown()
	}()

	broker.ListenAndServe(":4200")
}
