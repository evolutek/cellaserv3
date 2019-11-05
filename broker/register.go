package broker

import (
	"encoding/json"

	cellaserv "bitbucket.org/evolutek/cellaserv3-protobuf"
)

// Add service to services map
func (b *Broker) handleRegister(c *client, msg *cellaserv.Register) {
	name := msg.Name
	ident := msg.Identification

	c.mtx.Lock()
	defer c.mtx.Unlock()

	b.servicesMtx.Lock()
	defer b.servicesMtx.Unlock()

	if _, ok := b.services[name]; !ok {
		b.services[name] = make(map[string]*service)
	}

	registeredService := newService(c, name, ident)

	b.logger.Infof("New service: %s", registeredService)

	// Check for duplicate services
	if s, ok := b.services[name][ident]; ok {
		s.logger.Warnf("Service is replaced.")

		pubJSON, _ := json.Marshal(s.JSONStruct())
		b.cellaservPublishBytes(logLostService, pubJSON)

		service_client := s.client

		for i, ss := range service_client.services {
			if ss.Name == name && ss.Identification == ident {
				// Remove from slice
				service_client.services[i] = service_client.services[len(service_client.services)-1]
				service_client.services = service_client.services[:len(service_client.services)-1]
				break
			}
		}
	} else {
		// Sanity checks
		if ident == "" {
			if len(b.services[name]) >= 1 {
				b.logger.Warn("New service has no identification but there is already a service with an identification.")
			}
		} else {
			if _, ok = b.services[name][""]; ok {
				b.logger.Warn("New service has an identification but there is already a service without an identification")
			}
		}
	}

	// This makes all requests go to the new service
	b.services[name][ident] = registeredService

	// Keep track of origin client in order to remove it when the connection is closed
	c.services = append(c.services, registeredService)

	// Special case for the internal cellaserv service.
	if name == "cellaserv" {
		close(b.startedWithCellaserv)
	}

	// Publish new service event
	pubJSON, _ := json.Marshal(registeredService.JSONStruct())
	b.cellaservPublishBytes(logNewService, pubJSON)
}
