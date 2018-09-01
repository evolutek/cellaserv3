package broker

import (
	"bitbucket.org/evolutek/cellaserv2-protobuf"
	"encoding/json"
	"net"
)

// Add service to service map
func handleRegister(conn net.Conn, msg *cellaserv.Register) {
	name := msg.GetName()
	ident := msg.GetIdentification()
	service := newService(conn, name, ident)
	log.Info("[Services] New %s/%s", name, ident)

	if _, ok := services[name]; !ok {
		services[name] = make(map[string]*Service)
	}

	// Check for duplicate services
	if s, ok := services[name][ident]; ok {
		log.Warning("[Services] Replace %s", s)

		pub_json, _ := json.Marshal(s.JSONStruct())
		cellaservPublish(logLostService, pub_json)

		sc := servicesConn[s.Conn]
		for i, ss := range sc {
			if ss.Name == name && ss.Identification == ident {
				// Remove from slice
				sc[i] = sc[len(sc)-1]
				servicesConn[s.Conn] = sc[:len(sc)-1]

				// Clear key from map if list is empty
				if len(servicesConn[s.Conn]) == 0 {
					delete(servicesConn, s.Conn)
				}
			}
		}
	} else {
		// Sanity checks
		if ident == "" {
			if len(services[name]) >= 1 {
				log.Warning("[Service] New service have no identification but " +
					"there is already a service with an identification.")
			}
		} else {
			if _, ok = services[name][""]; ok {
				log.Warning("[Service] New service have an identification but " +
					"there is already a service without an identification")
			}
		}
	}

	// This makes all requests go to the new service
	services[name][ident] = service

	// Keep track of origin connection in order to remove when the connection is closed
	servicesConn[conn] = append(servicesConn[conn], service)

	// Publish new service data
	pub_json, _ := json.Marshal(service.JSONStruct())
	cellaservPublish(logNewService, pub_json)

	pub_json, _ = json.Marshal(connNameJSON{conn.RemoteAddr().String(), connDescribe(conn)})
	cellaservPublish(logConnRename, pub_json)
}

// vim: set nowrap tw=100 noet sw=8:
