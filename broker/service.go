package broker

import (
	"sync"

	"bitbucket.org/evolutek/cellaserv3/common"
	logging "github.com/op/go-logging"
)

type service struct {
	client         *client
	Name           string
	Identification string
	spiesMtx       sync.RWMutex
	spies          []*client
	logger         *logging.Logger
}

type ServiceJSON struct {
	Addr           string
	Name           string
	Identification string
}

func (s *service) String() string {
	if s.Identification != "" {
		return s.Name + "/" + s.Identification
	}
	return s.Name
}

// JSONStruct creates a struc good for JSON encoding.
func (s *service) JSONStruct() *ServiceJSON {
	return &ServiceJSON{
		Addr:           s.client.conn.RemoteAddr().String(),
		Name:           s.Name,
		Identification: s.Identification,
	}
}

func (s *service) sendMessage(msg []byte) {
	// No locking, multiple goroutine can write to a conn
	err := common.SendRawMessage(s.client.conn, msg)
	if err != nil {
		s.logger.Errorf("Could not send message: %s", err)
	}
}

func newService(c *client, name string, ident string) *service {
	s := &service{
		client:         c,
		Name:           name,
		Identification: ident,
	}
	s.logger = common.NewLogger(s.String())
	return s
}
