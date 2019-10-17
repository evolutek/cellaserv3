package client

import (
	"encoding/json"
	"fmt"

	cellaserv "bitbucket.org/evolutek/cellaserv3-protobuf"
)

type RequestHandlerFunc func(*cellaserv.Request) (interface{}, error)

type EventHandlerFunc func(*cellaserv.Publish)

type service struct {
	Name           string
	Identification string

	requestHandlers map[string](RequestHandlerFunc)
	eventHandlers   map[string](EventHandlerFunc)
}

func (s *service) String() string {
	return fmt.Sprintf("%s[%s]", s.Name, s.Identification)
}

// NewService returns an initialized Service instance
func (c *Client) NewService(name string, identification string) *service {
	return &service{
		Name:            name,
		Identification:  identification,
		requestHandlers: make(map[string](RequestHandlerFunc)),
		eventHandlers:   make(map[string](EventHandlerFunc)),
	}
}

func (s *service) HandleRequestFunc(action string, f RequestHandlerFunc) {
	s.requestHandlers[action] = f
}

func (s *service) HandleEventFunc(event string, f EventHandlerFunc) {
	s.eventHandlers[event] = f
}

func (s *service) handleRequest(req *cellaserv.Request, method string) ([]byte, error) {
	// Find handler
	handle, ok := s.requestHandlers[method]
	if !ok {
		return nil, fmt.Errorf("No such method: %s", method)
	}

	// Call handler
	reply, err := handle(req)
	if err != nil {
		return nil, err
	}

	// Marshal reply object as JSON
	replyBytes, err := json.Marshal(reply)
	if err != nil {
		return nil, err
	}
	return replyBytes, nil

}
