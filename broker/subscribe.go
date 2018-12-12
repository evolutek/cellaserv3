package broker

import (
	"encoding/json"
	"strings"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
)

type logSubscriberJSON struct {
	Event   string
	SubAddr string
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

	pubJSON, _ := json.Marshal(logSubscriberJSON{sub.Event, c.String()})
	b.cellaservPublish(logNewSubscriber, pubJSON)
}
