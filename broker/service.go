package broker

import (
	"net"

	"github.com/evolutek/cellaserv3/common"
)

type service struct {
	Conn           net.Conn
	Name           string
	Identification string
	Spies          []net.Conn
}

type serviceJSON struct {
	Addr           string
	Name           string
	Identification string
}

func newService(conn net.Conn, name string, ident string) *service {
	s := &service{conn, name, ident, nil}
	return s
}

func (s *service) String() string {
	if s.Identification != "" {
		return s.Name + "/" + s.Identification
	}
	return s.Name
}

// JSONStruct creates a struc good for JSON encoding.
func (s *service) JSONStruct() *serviceJSON {
	return &serviceJSON{
		Addr:           s.Conn.RemoteAddr().String(),
		Name:           s.Name,
		Identification: s.Identification,
	}
}

func (s *service) sendMessage(msg []byte) {
	common.SendRawMessage(s.Conn, msg)
}
