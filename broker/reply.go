package broker

import (
	cellaserv "github.com/evolutek/cellaserv3-protobuf"
	log "github.com/sirupsen/logrus"
)

func (b *Broker) handleReply(c *client, msgRaw []byte, rep *cellaserv.Reply) {
	id := rep.Id

	logger := log.WithFields(log.Fields{
		"module":     "reply",
		"src_client": c.String(),
		"id":         id,
	})

	b.reqIdsMtx.RLock()
	reqTrack, ok := b.reqIds[id]
	b.reqIdsMtx.RUnlock()
	if !ok {
		logger.Errorf("Could not find a matching request.")
		return
	}
	b.reqIdsMtx.Lock()
	delete(b.reqIds, id)
	b.reqIdsMtx.Unlock()

	reqTrack.timer.Stop()

	// Track reply latency
	reqTrack.latencyObserver.ObserveDuration()

	// Forward reply to spies
	// TODO(halfr): make sure timeouts are also sent to spies
	for _, spy := range reqTrack.spies {
		logger.Debugf("Sending reply to spy %s", spy.conn)
		b.sendRawMessage(spy.conn, msgRaw)
	}

	logger.Infof("Sending reply to destingation client: %s", reqTrack.sender)
	b.sendRawMessage(reqTrack.sender.conn, msgRaw)
}
