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

func handleRequest(conn net.Conn, msgRaw []byte, req *cellaserv.Request) {
	log.Info("[Request] Incoming from %s", conn.RemoteAddr())

	name := req.GetServiceName()
	method := req.GetMethod()
	id := req.GetId()
	ident := req.GetServiceIdentification()

	if ident != "" {
		log.Debug("[Request] id:%d %s[%s].%s", id, name, ident, method)
	} else {
		log.Debug("[Request] id:%d %s.%s", id, name, method)
	}

	if name == "cellaserv" {
		cellaservRequest(conn, req)
		return
	}

	idents, ok := services[name]
	if !ok || len(idents) == 0 {
		log.Warning("[Request] id:%d No such service: %s", id, name)
		sendReplyError(conn, req, cellaserv.Reply_Error_NoSuchService)
		return
	}
	srvc, ok := idents[ident]
	if !ok {
		log.Warning("[Request] id:%d No such identification for service %s: %s",
			id, name, ident)
		sendReplyError(conn, req, cellaserv.Reply_Error_InvalidIdentification)
		return
	}

	// Handle timeouts
	handleTimeout := func() {
		_, ok := reqIds[id]
		if ok {
			log.Error("[Request] id:%d Timeout of %s", id, srvc)
			sendReplyError(conn, req, cellaserv.Reply_Error_Timeout)
		}
	}
	timer := time.AfterFunc(5*time.Second, handleTimeout)

	// The ID is used to track the sender of the request
	reqIds[id] = &requestTracking{conn, timer, srvc.Spies}

	srvc.sendMessage(msgRaw)

	// Forward message to the spies of this service
	for _, spy := range srvc.Spies {
		common.SendRawMessage(spy, msgRaw)
	}
}
