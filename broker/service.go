package broker

import (
	"net"
	"sync"

	"bitbucket.org/evolutek/cellaserv3/common"
	logging "github.com/op/go-logging"
)

type service struct {
	Conn           net.Conn
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

func newService(conn net.Conn, name string, ident string) *service {
	s := &service{
		Conn:           conn,
		Name:           name,
		Identification: ident,
	}
	s.logger = common.NewLogger(s.String())
	return s
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
		Addr:           s.Conn.RemoteAddr().String(),
		Name:           s.Name,
		Identification: s.Identification,
	}
}

func (s *service) sendMessage(msg []byte) {
	// No locking, multiple goroutine can write to a conn
	err := common.SendRawMessage(s.Conn, msg)
	if err != nil {
		s.logger.Errorf("Could not send message: %s", err)
	}
}

// The service lock must be held by the caller
func (s *service) removeSpy(spy *client) {
}
