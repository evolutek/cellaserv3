package broker

import (
	"encoding/json"
	"net"
	"strings"

	"bitbucket.org/evolutek/cellaserv2-protobuf"
)

type logSubscriberJSON struct {
	Event   string
	SubAddr string
}

func (b *Broker) handleSubscribe(conn net.Conn, sub *cellaserv.Subscribe) {
	b.logger.Info("[Subscribe] %s subscribes to %s", conn.RemoteAddr(), *sub.Event)
	if strings.Contains(*sub.Event, "*") {
		b.subscriberMatchMap[*sub.Event] = append(b.subscriberMatchMap[*sub.Event], conn)
	} else {
		b.subscriberMap[*sub.Event] = append(b.subscriberMap[*sub.Event], conn)
	}

	pubJSON, _ := json.Marshal(logSubscriberJSON{*sub.Event, conn.RemoteAddr().String()})
	b.cellaservPublish(logNewSubscriber, pubJSON)
}
