package broker

import (
	"fmt"
	"time"

	cellaserv "bitbucket.org/evolutek/cellaserv3-protobuf"
	"bitbucket.org/evolutek/cellaserv3/common"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type requestTracking struct {
	sender          *client
	timer           *time.Timer
	spies           []*client
	latencyObserver *prometheus.Timer
}

func (b *Broker) handleRequest(c *client, msgRaw []byte, req *cellaserv.Request) {
	name := req.ServiceName
	method := req.Method
	id := req.Id
	ident := req.ServiceIdentification

	logger := log.WithFields(log.Fields{
		"module": "request",
		"client": c.String(),
		"id":     id,
		"method": method,
	})

	idents, ok := b.services[name]
	if !ok || len(idents) == 0 {
		logger.Warnln("No such service with this name.")
		b.sendReplyError(c, req, cellaserv.Reply_Error_NoSuchService)
		return
	}
	srvc, ok := idents[ident]
	if !ok {
		logger.Warnln("No such service with that identification.")
		b.sendReplyError(c, req, cellaserv.Reply_Error_InvalidIdentification)
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

			logger.Errorln("Timeout.")
			b.sendReplyError(c, req, cellaserv.Reply_Error_Timeout)
		}
	}
	timer := time.AfterFunc(b.Options.RequestTimeoutSec*time.Second, handleTimeout)

	// The ID is used to track the sender of the request
	reqTrack := &requestTracking{
		sender:          c,
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
		err := common.SendRawMessage(spy.conn, msgRaw)
		if err != nil {
			logger.Warnf("Could not forward request to spy %s: %s", spy, err)
		}
	}
	srvc.spiesMtx.RUnlock()
}

func (b *Broker) GetRequestSender(req *cellaserv.Request) (*client, error) {
	b.reqIdsMtx.RLock()
	defer b.reqIdsMtx.RUnlock()

	// Use request ID to find the sender client
	tracker, ok := b.reqIds[req.Id]
	if !ok {
		return nil, fmt.Errorf("Could not find request %x", req.Id)
	}
	return tracker.sender, nil
}
