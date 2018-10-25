package broker

import (
	"net"
	"path/filepath"
	"strings"

	"bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/evolutek/cellaserv3/common"
)

func (b *Broker) handlePublish(conn net.Conn, msgBytes []byte, pub *cellaserv.Publish) {
	b.logger.Info("[Publish] %s publishes %s", b.connDescribe(conn), *pub.Event)
	b.doPublish(msgBytes, pub)
}

func (b *Broker) doPublish(msgBytes []byte, pub *cellaserv.Publish) {
	event := *pub.Event

	// Logging
	b.logger.Debug("[Publish] Publishing %s", event)

	// Handle log publishes
	if strings.HasPrefix(event, "b.logger.") {
		var data string
		if pub.Data != nil {
			data = string(pub.Data)
		}
		event := (*pub.Event)[4:] // Strip 'b.logger.' prefix
		common.LogEvent(event, data)
	}

	// Holds subscribers for this publish
	var subs []net.Conn

	// Handle glob susbscribers
	for pattern, cons := range b.subscriberMatchMap {
		matched, _ := filepath.Match(pattern, event)
		if matched {
			subs = append(subs, cons...)
		}
	}

	// Add exact matches
	subs = append(subs, b.subscriberMap[event]...)

	for _, connSub := range subs {
		b.logger.Debug("[Publish] Forwarding %s to %s", pub.GetEvent(), b.connDescribe(connSub))
		common.SendRawMessage(connSub, msgBytes)
	}
}
