package broker

import (
	"net"

	"bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/evolutek/cellaserv3/common"
)

func (b *Broker) handleReply(conn net.Conn, msgRaw []byte, rep *cellaserv.Reply) {
	id := *rep.Id
	b.logger.Info("[Reply] id:%d reply from %s", id, conn.RemoteAddr())

	reqTrack, ok := b.reqIds[id]
	if !ok {
		b.logger.Error("[Reply] Unknown ID: %d", id)
		return
	}
	delete(b.reqIds, id)

	// Forward reply to spies
	for _, spy := range reqTrack.spies {
		common.SendRawMessage(spy, msgRaw)
	}

	reqTrack.timer.Stop()
	b.logger.Debug("[Reply] Forwarding to %s", reqTrack.sender.RemoteAddr())
	common.SendRawMessage(reqTrack.sender, msgRaw)
}
