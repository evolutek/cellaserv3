package broker

import (
	"net"
	"time"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"bitbucket.org/evolutek/cellaserv3/common"
	"github.com/prometheus/client_golang/prometheus"
)

type requestTracking struct {
	sender          net.Conn
	timer           *time.Timer
	spies           []*client
	latencyObserver *prometheus.Timer
}

func (b *Broker) handleRequest(conn net.Conn, msgRaw []byte, req *cellaserv.Request) {
	name := req.GetServiceName()
	method := req.GetMethod()
	id := req.GetId()
	ident := req.GetServiceIdentification()

	b.logger.Infof("[Request] id:%x %s → %s[%s].%s", id, conn.RemoteAddr(), name, ident, method)

	if ident != "" {
		b.logger.Debugf("[Request] id:%d %s[%s].%s", id, name, ident, method)
	} else {
		b.logger.Debugf("[Request] id:%d %s.%s", id, name, method)
	}

	if name == "cellaserv" {
		b.cellaservRequest(conn, req)
		return
	}

	idents, ok := b.services[name]
	if !ok || len(idents) == 0 {
		b.logger.Warningf("[Request] id:%d No such service: %s", id, name)
		b.sendReplyError(conn, req, cellaserv.Reply_Error_NoSuchService)
		return
	}
	srvc, ok := idents[ident]
	if !ok {
		b.logger.Warningf("[Request] id:%d No such identification for service %s: %s",
			id, name, ident)
		b.sendReplyError(conn, req, cellaserv.Reply_Error_InvalidIdentification)
		return
	}

	// Handle timeouts
	handleTimeout := func() {
		b.reqIdsMtx.RLock()
		_, ok := b.reqIds[id]
		b.reqIdsMtx.RUnlock()
		if ok {
			b.reqIdsMtx.Lock()
			delete(b.reqIds, id)
			b.reqIdsMtx.Unlock()

			b.logger.Errorf("[Request] id:%x Timeout of %s", id, srvc)
			b.sendReplyError(conn, req, cellaserv.Reply_Error_Timeout)
		}
	}
	timer := time.AfterFunc(b.Options.RequestTimeoutSec*time.Second, handleTimeout)

	// The ID is used to track the sender of the request
	reqTrack := &requestTracking{
		sender:          conn,
		timer:           timer,
		spies:           srvc.spies,
		latencyObserver: prometheus.NewTimer(b.Monitoring.requests.WithLabelValues(req.GetServiceName(), req.GetServiceIdentification(), req.GetMethod()))}
	b.reqIdsMtx.Lock()
	b.reqIds[id] = reqTrack
	b.reqIdsMtx.Unlock()

	srvc.sendMessage(msgRaw)

	// Forward message to the spies of this service
	srvc.spiesMtx.RLock()
	for _, spy := range srvc.spies {
		common.SendRawMessage(spy.conn, msgRaw)
	}
	srvc.spiesMtx.RUnlock()
}
