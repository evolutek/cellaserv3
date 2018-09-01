package broker

import "net"

type Service struct {
	Conn           net.Conn
	Name           string
	Identification string
	Spies          []net.Conn
}

type ServiceJSON struct {
	Addr           string
	Name           string
	Identification string
}

func newService(conn net.Conn, name string, ident string) *Service {
	s := &Service{conn, name, ident, nil}
	return s
}

func (s *Service) String() string {
	if s.Identification != "" {
		return s.Name + "/" + s.Identification
	}
	return s.Name
}

// JSONStruct creates a struc good for JSON encoding.
func (s *Service) JSONStruct() *ServiceJSON {
	return &ServiceJSON{
		Addr:           s.Conn.RemoteAddr().String(),
		Name:           s.Name,
		Identification: s.Identification,
	}
}

func (s *Service) sendMessage(msg []byte) {
	sendRawMessage(s.Conn, msg)
}

// vim: set nowrap tw=100 noet sw=8:
