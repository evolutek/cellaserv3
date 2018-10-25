package broker

import (
	"net"
	"time"

	"bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/evolutek/cellaserv3/common"
)

type requestTracking struct {
	sender net.Conn
	timer  *time.Timer
	spies  []net.Conn
}

func (b *Broker) handleRequest(conn net.Conn, msgRaw []byte, req *cellaserv.Request) {
	b.logger.Info("[Request] Incoming from %s", conn.RemoteAddr())

	name := req.GetServiceName()
	method := req.GetMethod()
	id := req.GetId()
	ident := req.GetServiceIdentification()

	if ident != "" {
		b.logger.Debug("[Request] id:%d %s[%s].%s", id, name, ident, method)
	} else {
		b.logger.Debug("[Request] id:%d %s.%s", id, name, method)
	}

	if name == "cellaserv" {
		b.cellaservRequest(conn, req)
		return
	}

	idents, ok := b.Services[name]
	if !ok || len(idents) == 0 {
		b.logger.Warning("[Request] id:%d No such service: %s", id, name)
		b.sendReplyError(conn, req, cellaserv.Reply_Error_NoSuchService)
		return
	}
	srvc, ok := idents[ident]
	if !ok {
		b.logger.Warning("[Request] id:%d No such identification for service %s: %s",
			id, name, ident)
		b.sendReplyError(conn, req, cellaserv.Reply_Error_InvalidIdentification)
		return
	}

	// Handle timeouts
	handleTimeout := func() {
		_, ok := b.reqIds[id]
		if ok {
			b.logger.Error("[Request] id:%d Timeout of %s", id, srvc)
			b.sendReplyError(conn, req, cellaserv.Reply_Error_Timeout)
		}
	}
	timer := time.AfterFunc(5*time.Second, handleTimeout)

	// The ID is used to track the sender of the request
	b.reqIds[id] = &requestTracking{conn, timer, srvc.Spies}

	srvc.sendMessage(msgRaw)

	// Forward message to the spies of this service
	for _, spy := range srvc.Spies {
		common.SendRawMessage(spy, msgRaw)
	}
}
