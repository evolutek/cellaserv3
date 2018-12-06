package broker

import (
	"net"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
)

func (b *Broker) handleReply(conn net.Conn, msgRaw []byte, rep *cellaserv.Reply) {
	id := rep.Id
	b.logger.Infof("[Reply] id:%d reply from %s", id, conn.RemoteAddr())

	reqTrack, ok := b.reqIds[id]
	if !ok {
		b.logger.Errorf("[Reply] Unknown ID: %d", id)
		return
	}
	delete(b.reqIds, id)

	// Track reply latency
	reqTrack.latencyObserver.ObserveDuration()

	// Forward reply to spies
	for _, spy := range reqTrack.spies {
		b.sendRawMessage(spy, msgRaw)
	}

	reqTrack.timer.Stop()
	b.logger.Debugf("[Reply] Forwarding to %s", reqTrack.sender.RemoteAddr())
	b.sendRawMessage(reqTrack.sender, msgRaw)
}
