package broker

import (
	"encoding/json"
	"net"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
)

// Add service to services map
func (b *Broker) handleRegister(conn net.Conn, msg *cellaserv.Register) {
	name := msg.GetName()
	ident := msg.GetIdentification()
	b.logger.Infof("[Services] New %s/%s", name, ident)

	c, ok := b.getClientByConn(conn)
	if !ok {
		b.logger.Warningf("[Register] Connection disconected while handling register: %s", conn)
		return
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()

	b.servicesMtx.Lock()
	defer b.servicesMtx.Unlock()

	if _, ok := b.services[name]; !ok {
		b.services[name] = make(map[string]*service)
	}

	registeredService := newService(conn, name, ident)

	// Check for duplicate services
	if s, ok := b.services[name][ident]; ok {
		b.logger.Warningf("[Services] Replace %s", s)

		pubJSON, _ := json.Marshal(s.JSONStruct())
		b.cellaservPublish(logLostService, pubJSON)

		for i, ss := range c.services {
			if ss.Name == name && ss.Identification == ident {
				// Remove from slice
				c.services[i] = c.services[len(c.services)-1]
				c.services = c.services[:len(c.services)-1]
				break
			}
		}
	} else {
		// Sanity checks
		if ident == "" {
			if len(b.services[name]) >= 1 {
				b.logger.Warning("[Service] New service have no identification but there is already a service with an identification.")
			}
		} else {
			if _, ok = b.services[name][""]; ok {
				b.logger.Warning("[Service] New service have an identification but there is already a service without an identification")
			}
		}
	}

	// This makes all requests go to the new service
	b.services[name][ident] = registeredService

	// Keep track of origin client in order to remove it when the connection is closed
	c.services = append(c.services, registeredService)

	// Publish new service events
	pubJSON, _ := json.Marshal(registeredService.JSONStruct())
	b.cellaservPublish(logNewService, pubJSON)

	pubJSON, _ = json.Marshal(ConnectionJSON{conn.RemoteAddr().String(), b.connDescribe(conn)})
	b.cellaservPublish(logConnRename, pubJSON)
}
