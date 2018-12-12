package broker

import (
	"net"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
)

func (b *Broker) handleReply(conn net.Conn, msgRaw []byte, rep *cellaserv.Reply) {
	id := rep.Id

	b.reqIdsMtx.RLock()
	reqTrack, ok := b.reqIds[id]
	b.reqIdsMtx.RUnlock()
	if !ok {
		b.logger.Errorf("[Reply] Unknown ID: %x", id)
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
		b.sendRawMessage(spy.conn, msgRaw)
	}

	b.logger.Infof("[Reply] id:%x %s → %s", id, conn.RemoteAddr(), reqTrack.sender.RemoteAddr())
	b.sendRawMessage(reqTrack.sender, msgRaw)
}
