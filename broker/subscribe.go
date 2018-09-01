package broker

import (
	"bitbucket.org/evolutek/cellaserv2-protobuf"
	"encoding/json"
	"net"
	"strings"
)

type LogSubscriberJSON struct {
	Event   string
	SubAddr string
}

func handleSubscribe(conn net.Conn, sub *cellaserv.Subscribe) {
	log.Info("[Subscribe] %s subscribes to %s", conn.RemoteAddr(), *sub.Event)
	if strings.Contains(*sub.Event, "*") {
		subscriberMatchMap[*sub.Event] = append(subscriberMatchMap[*sub.Event], conn)
	} else {
		subscriberMap[*sub.Event] = append(subscriberMap[*sub.Event], conn)
	}

	pub_json, _ := json.Marshal(LogSubscriberJSON{*sub.Event, conn.RemoteAddr().String()})
	cellaservPublish(logNewSubscriber, pub_json)
}

// vim: set nowrap tw=100 noet sw=8:
