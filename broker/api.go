package broker

import (
	"bitbucket.org/evolutek/cellaserv3/broker/cellaserv/api"
)

func (b *Broker) GetEventsJSON() api.EventsJSON {
	events := make(api.EventsJSON)

	fillMap := func(subMap map[string][]*client) {
		for event, clients := range subMap {
			var connSlice []string
			for _, c := range clients {
				connSlice = append(connSlice, c.conn.RemoteAddr().String())
			}
			events[event] = connSlice
		}
	}

	b.subscriberMapMtx.RLock()
	fillMap(b.subscriberMap)
	b.subscriberMapMtx.RUnlock()

	b.subscriberMatchMapMtx.RLock()
	fillMap(b.subscriberMatchMap)
	b.subscriberMatchMapMtx.RUnlock()

	return events
}

func (b *Broker) GetClientsJSON() []api.ClientJSON {
	var conns []api.ClientJSON
	b.mapClientIdToClient.Range(func(key, value interface{}) bool {
		c := value.(*client)
		conn := c.JSONStruct()
		conns = append(conns, conn)
		return true
	})
	return conns
}

func (b *Broker) GetServicesJSON() []api.ServiceJSON {
	// Fix static empty slice that is "null" in JSON
	// A dynamic empty slice is []
	servicesList := make([]api.ServiceJSON, 0)
	for _, names := range b.services {
		for _, s := range names {
			servicesList = append(servicesList, *s.JSONStruct())
		}
	}
	return servicesList
}
