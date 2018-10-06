package broker

import (
	"encoding/json"
	"net"

	"bitbucket.org/evolutek/cellaserv2-protobuf"
)

// Add service to services map
func handleRegister(conn net.Conn, msg *cellaserv.Register) {
	name := msg.GetName()
	ident := msg.GetIdentification()
	log.Info("[Services] New %s/%s", name, ident)

	if _, ok := services[name]; !ok {
		services[name] = make(map[string]*service)
	}

	// Check for duplicate services
	if s, ok := services[name][ident]; ok {
		log.Warning("[Services] Replace %s", s)

		pubJSON, _ := json.Marshal(s.JSONStruct())
		cellaservPublish(logLostService, pubJSON)

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

	registeredService := newService(conn, name, ident)

	// This makes all requests go to the new service
	services[name][ident] = registeredService

	// Keep track of origin connection in order to remove it when the connection is closed
	servicesConn[conn] = append(servicesConn[conn], registeredService)

	// Publish new service events
	pubJSON, _ := json.Marshal(registeredService.JSONStruct())
	cellaservPublish(logNewService, pubJSON)

	pubJSON, _ = json.Marshal(ConnNameJSON{conn.RemoteAddr().String(), connDescribe(conn)})
	cellaservPublish(logConnRename, pubJSON)
}
