package broker

import (
	"encoding/json"
	"path/filepath"
	"strings"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/golang/protobuf/proto"
)

const (
	logClientName     = "log.cellaserv.client-name"
	logLostClient     = "log.cellaserv.lost-client"
	logLostService    = "log.cellaserv.lost-service"
	logLostSubscriber = "log.cellaserv.lost-subscriber"
	logNewClient      = "log.cellaserv.new-client"
	logNewService     = "log.cellaserv.new-service"
	logNewSubscriber  = "log.cellaserv.new-subscriber"
)

func (b *Broker) handlePublish(c *client, msgBytes []byte, pub *cellaserv.Publish) {
	c.logger.Infof("Publishes event %q", pub.Event)
	b.doPublish(msgBytes, pub)
}

func (b *Broker) doPublish(msgBytes []byte, pub *cellaserv.Publish) {
	// Set of subscribers for this publish
	subs := make(map[*client]bool)

	// Handle log publishes
	if b.Options.PublishLoggingEnabled && strings.HasPrefix(pub.Event, "log.") {
		loggingEvent := strings.TrimPrefix(pub.Event, "log.")
		data := string(pub.Data) // expect data to be utf8
		b.handleLoggingPublish(loggingEvent, data)
	}

	// Handle glob susbscribers
	b.subscriberMapMtx.RLock()
	for pattern, clients := range b.subscriberMatchMap {
		matched, _ := filepath.Match(pattern, pub.Event)
		if matched {
			for _, client := range clients {
				subs[client] = true
			}
		}
	}
	b.subscriberMapMtx.RUnlock()

	// Add exact matches
	for _, client := range b.subscriberMap[pub.Event] {
		subs[client] = true
	}

	for c := range subs {
		c.logger.Debugf("Receives event %q", pub.Event)
		b.sendRawMessage(c.conn, msgBytes)
	}
}

// cellaservPublishBytes sends a publish message from cellaserv
// TODO: use the cellaserv internal service logger
func (b *Broker) cellaservPublishBytes(event string, data []byte) {
	b.logger.Debugf("Publishes event %q", event)

	pub := &cellaserv.Publish{Event: event}
	if data != nil {
		pub.Data = data
	}
	pubBytes, err := proto.Marshal(pub)
	if err != nil {
		b.logger.Errorf("Could not marshal event: %s", err)
		return
	}
	msgType := cellaserv.Message_Publish
	msg := &cellaserv.Message{Type: msgType, Content: pubBytes}
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		b.logger.Errorf("Could not marshal event: %s", err)
		return
	}

	b.doPublish(msgBytes, pub)
}

func (b *Broker) cellaservPublish(event string, obj interface{}) {
	pubData, err := json.Marshal(obj)
	if err != nil {
		b.logger.Errorf("Unable to marshal publish: %s", err)
		return
	}
	b.cellaservPublishBytes(event, pubData)
}
