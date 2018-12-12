package broker

import (
	"strings"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
)

type logSubscriberJSON struct {
	Event       string
	SubClientId string
}

func (b *Broker) handleSubscribe(c *client, sub *cellaserv.Subscribe) {
	b.logger.Infof("[Subscribe] %s subscribes to %s", c, sub.Event)
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
