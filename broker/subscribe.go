package broker

import (
	"strings"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
)

type logSubscriberJSON struct {
	Event  string `json:"event"`
	Client string `json:"client"`
}

func (b *Broker) handleSubscribe(c *client, sub *cellaserv.Subscribe) {
	b.logger.Infof("[Subscribe] %s subscribes to %s", c, sub.Event)

	// Check for duplicate subscribes by the client
	c.mtx.Lock()
	defer c.mtx.Unlock()
	present := false
	for _, pattern := range c.subscribes {
		if pattern == sub.Event {
			present = true
			break
		}
	}
	if present {
		b.logger.Infof("[Subscribe] %s is already subscribed to %s", c, sub.Event)
		return
	}
	c.subscribes = append(c.subscribes, sub.Event)

	if strings.Contains(sub.Event, "*") {
		b.subscriberMatchMapMtx.Lock()
		b.subscriberMatchMap[sub.Event] = append(b.subscriberMatchMap[sub.Event], c)
		b.subscriberMatchMapMtx.Unlock()
	} else {
		b.subscriberMapMtx.Lock()
		b.subscriberMap[sub.Event] = append(b.subscriberMap[sub.Event], c)
		b.subscriberMapMtx.Unlock()
	}

	b.cellaservPublish(logNewSubscriber, logSubscriberJSON{sub.Event, c.id})
}
