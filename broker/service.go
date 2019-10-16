package broker

import (
	"fmt"
	"sync"

	"bitbucket.org/evolutek/cellaserv3/broker/cellaserv/api"
	"bitbucket.org/evolutek/cellaserv3/common"
	log "github.com/sirupsen/logrus"
)

// A service is a unique entity attached to a cellaserv client that can
// received requests.
type service struct {
	client         *client
	Name           string
	Identification string
	spiesMtx       sync.RWMutex
	spies          []*client
	logger         common.Logger
}

func (s *service) String() string {
	return fmt.Sprintf("%s[%s]", s.Name, s.Identification)
}

// JSONStruct creates a struc good for JSON encoding.
func (s *service) JSONStruct() *api.ServiceJSON {
	return &api.ServiceJSON{
		Client:         s.client.id,
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

// spyByRequest finds the sender of the request and add it to the
func (b *Broker) SpyService(c *client, srvc *service) {
	srvc.logger.Debugf("client %s spies on service %s", c, srvc)

	srvc.spiesMtx.Lock()
	srvc.spies = append(srvc.spies, c)
	srvc.spiesMtx.Unlock()

	c.mtx.Lock()
	c.spying = append(c.spying, srvc)
	c.mtx.Unlock()
}

// GetService returns the service identified by the name and identification in
// argument, or an error if not found.
func (b *Broker) GetService(name string, identification string) (srvc *service, err error) {
	var ok bool
	b.servicesMtx.RLock()
	srvc, ok = b.services[name][identification]
	b.servicesMtx.RUnlock()
	if !ok {
		err = fmt.Errorf("No such service: %s[%s]", name, identification)
	}
	return
}

func newService(c *client, name string, ident string) *service {
	// Setup logger
	logger := log.WithFields(log.Fields{"module": "service",
		"name":           name,
		"identification": ident})

	// Create service
	return &service{
		client:         c,
		Name:           name,
		Identification: ident,
		logger:         logger,
	}
}
