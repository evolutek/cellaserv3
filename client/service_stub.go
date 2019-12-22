package client

import (
	"encoding/json"
	"fmt"

	cellaserv "bitbucket.org/evolutek/cellaserv3-protobuf"
)

type serviceStub struct {
	name           string
	identification string

	client *Client
}

func (s *serviceStub) String() string {
	return fmt.Sprintf("%s[%s]", s.name, s.identification)
}

func (s *serviceStub) sendRequest(req *cellaserv.Request) ([]byte, error) {
	s.client.logger.Debugf("Sending request %s[%s].%s(%s)", req.ServiceName, req.ServiceIdentification, req.Method, req.Data)

	reply := s.client.sendRequestWaitForReply(req)

	// Check for errors
	replyError := reply.GetError()
	if replyError != nil {
		s.client.logger.Errorf("Received reply error: %s", replyError.String())
		return nil, fmt.Errorf(replyError.String())
	}

	return reply.GetData(), nil
}

func (s *serviceStub) RequestNoData(method string) ([]byte, error) {
	// Create Request
	req := &cellaserv.Request{
		ServiceName:           s.name,
		ServiceIdentification: s.identification,
		Method:                method,
		// Id set by client
	}

	return s.sendRequest(req)
}

func (s *serviceStub) Request(method string, data interface{}) ([]byte, error) {
	// Serialize request payload
	dataBytes, err := json.Marshal(data)
	if err != nil {
		panic(fmt.Sprintf("Could not marshal to JSON: %v", data))
	}

	// Create Request
	req := &cellaserv.Request{
		Data:                  dataBytes,
		ServiceName:           s.name,
		ServiceIdentification: s.identification,
		Method:                method,
		// Id set by client
	}

	return s.sendRequest(req)
}

func (s *serviceStub) RequestRaw(method string, dataBytes []byte) ([]byte, error) {
	// Create Request
	req := &cellaserv.Request{
		Data:                  dataBytes,
		ServiceName:           s.name,
		ServiceIdentification: s.identification,
		Method:                method,
		// Id set by client
	}

	return s.sendRequest(req)
}

func NewServiceStub(c *Client, name string, identification string) *serviceStub {
	return &serviceStub{
		name:           name,
		identification: identification,
		client:         c,
	}
}
