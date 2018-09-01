package broker

import (
	"bitbucket.org/evolutek/cellaserv2-protobuf"
	"net"
)

func handleReply(conn net.Conn, msgRaw []byte, rep *cellaserv.Reply) {
	id := *rep.Id
	log.Info("[Reply] id:%d reply from %s", id, conn.RemoteAddr())

	reqTrack, ok := reqIds[id]
	if !ok {
		log.Error("[Reply] Unknown ID: %d", id)
		return
	}
	delete(reqIds, id)

	// Forward reply to spies
	for _, spy := range reqTrack.spies {
		sendRawMessage(spy, msgRaw)
	}

	reqTrack.timer.Stop()
	log.Debug("[Reply] Forwarding to %s", reqTrack.sender.RemoteAddr())
	sendRawMessage(reqTrack.sender, msgRaw)
}
