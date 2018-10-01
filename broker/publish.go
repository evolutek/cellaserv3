package broker

import (
	"net"
	"path/filepath"
	"strings"

	"bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/evolutek/cellaserv3/common"
)

func handlePublish(conn net.Conn, msgBytes []byte, pub *cellaserv.Publish) {
	log.Info("[Publish] %s publishes %s", connDescribe(conn), *pub.Event)
	doPublish(msgBytes, pub)
}

func doPublish(msgBytes []byte, pub *cellaserv.Publish) {
	event := *pub.Event

	// Logging
	log.Debug("[Publish] Publishing %s", event)

	// Handle log publishes
	if strings.HasPrefix(event, "log.") {
		var data string
		if pub.Data != nil {
			data = string(pub.Data)
		}
		event := (*pub.Event)[4:] // Strip 'log.' prefix
		common.LogEvent(event, data)
	}

	// Holds subscribers for this publish
	var subs []net.Conn

	// Handle glob susbscribers
	for pattern, cons := range subscriberMatchMap {
		matched, _ := filepath.Match(pattern, event)
		if matched {
			subs = append(subs, cons...)
		}
	}

	// Add exact matches
	subs = append(subs, subscriberMap[event]...)

	for _, connSub := range subs {
		log.Debug("[Publish] Forwarding publish to %s", connDescribe(connSub))
		common.SendRawMessage(connSub, msgBytes)
	}
}
