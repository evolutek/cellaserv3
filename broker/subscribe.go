package broker

import (
	"bitbucket.org/evolutek/cellaserv2-protobuf"
	"encoding/json"
	"net"
	"strings"
)

type logSubscriberJSON struct {
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

	pubJSON, _ := json.Marshal(logSubscriberJSON{*sub.Event, conn.RemoteAddr().String()})
	cellaservPublish(logNewSubscriber, pubJSON)
}
