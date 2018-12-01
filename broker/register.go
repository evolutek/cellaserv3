package broker

import (
	"encoding/json"
	"net"

	"bitbucket.org/evolutek/cellaserv2-protobuf"
)

// Add service to services map
func (b *Broker) handleRegister(conn net.Conn, msg *cellaserv.Register) {
	name := msg.GetName()
	ident := msg.GetIdentification()
	b.logger.Info("[Services] New %s/%s", name, ident)

	if _, ok := b.services[name]; !ok {
		b.services[name] = make(map[string]*service)
	}

	// Check for duplicate services
	if s, ok := b.services[name][ident]; ok {
		b.logger.Warning("[Services] Replace %s", s)

		pubJSON, _ := json.Marshal(s.JSONStruct())
		b.cellaservPublish(logLostService, pubJSON)

		sc := b.servicesConn[s.Conn]
		for i, ss := range sc {
			if ss.Name == name && ss.Identification == ident {
				// Remove from slice
				sc[i] = sc[len(sc)-1]
				b.servicesConn[s.Conn] = sc[:len(sc)-1]

				// Clear key from map if list is empty
				if len(b.servicesConn[s.Conn]) == 0 {
					delete(b.servicesConn, s.Conn)
				}
			}
		}
	} else {
		// Sanity checks
		if ident == "" {
			if len(b.services[name]) >= 1 {
				b.logger.Warning("[Service] New service have no identification but " +
					"there is already a service with an identification.")
			}
		} else {
			if _, ok = b.services[name][""]; ok {
				b.logger.Warning("[Service] New service have an identification but " +
					"there is already a service without an identification")
			}
		}
	}

	registeredService := newService(conn, name, ident)

	// This makes all requests go to the new service
	b.services[name][ident] = registeredService

	// Keep track of origin connection in order to remove it when the connection is closed
	b.servicesConn[conn] = append(b.servicesConn[conn], registeredService)

	// Publish new service events
	pubJSON, _ := json.Marshal(registeredService.JSONStruct())
	b.cellaservPublish(logNewService, pubJSON)

	pubJSON, _ = json.Marshal(ConnectionJSON{conn.RemoteAddr().String(), b.connDescribe(conn)})
	b.cellaservPublish(logConnRename, pubJSON)
}
