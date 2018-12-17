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
	b.logger.Infof("[Publish] %s publishes %s", c, pub.Event)
	b.doPublish(msgBytes, pub)
}

func (b *Broker) doPublish(msgBytes []byte, pub *cellaserv.Publish) {
	// Holds subscribers for this publish
	var subs []*client

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
			subs = append(subs, clients...)
		}
	}
	b.subscriberMapMtx.RUnlock()

	// Add exact matches
	subs = append(subs, b.subscriberMap[pub.Event]...)

	for _, c := range subs {
		b.logger.Debugf("[Publish] %s → %s", pub.Event, c)
		b.sendRawMessage(c.conn, msgBytes)
	}
}

// cellaservPublishBytes sends a publish message from cellaserv
func (b *Broker) cellaservPublishBytes(event string, data []byte) {
	pub := &cellaserv.Publish{Event: event}
	if data != nil {
		pub.Data = data
	}
	pubBytes, err := proto.Marshal(pub)
	if err != nil {
		b.logger.Errorf("[Cellaserv] Could not marshal event: %s", err)
		return
	}
	msgType := cellaserv.Message_Publish
	msg := &cellaserv.Message{Type: msgType, Content: pubBytes}
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		b.logger.Errorf("[Cellaserv] Could not marshal event: %s", err)
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
