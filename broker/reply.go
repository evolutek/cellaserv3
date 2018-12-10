package broker

import (
	"net"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
)

func (b *Broker) handleReply(conn net.Conn, msgRaw []byte, rep *cellaserv.Reply) {
	id := rep.Id
	b.logger.Infof("[Reply] id:%d reply from %s", id, conn.RemoteAddr())

	b.reqIdsMtx.RLock()
	reqTrack, ok := b.reqIds[id]
	b.reqIdsMtx.RUnlock()
	if !ok {
		b.logger.Errorf("[Reply] Unknown ID: %d", id)
		return
	}
	b.reqIdsMtx.Lock()
	delete(b.reqIds, id)
	b.reqIdsMtx.Unlock()

	// Track reply latency
	reqTrack.latencyObserver.ObserveDuration()

	// Forward reply to spies
	// TODO(halfr): make sure timeouts are also sent to spies
	for _, spy := range reqTrack.spies {
		b.sendRawMessage(spy.conn, msgRaw)
	}

	reqTrack.timer.Stop()
	b.logger.Debugf("[Reply] Forwarding to %s", reqTrack.sender.RemoteAddr())
	b.sendRawMessage(reqTrack.sender, msgRaw)
}
