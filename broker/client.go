package broker

import (
	"encoding/json"
	"net"
	"sync"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"bitbucket.org/evolutek/cellaserv3/common"
	"github.com/golang/protobuf/proto"
)

// client represents a single connnection to cellaserv
type client struct {
	mtx      sync.Mutex // protects slices below
	conn     net.Conn   // connection of this client
	id       string     // unique id for this client
	name     string     // name of this client
	spying   []*service // services spied by this client
	services []*service // services registered by this clietn
}

func (c *client) String() string {
	if c.name != "" {
		return c.name
	}
	return c.conn.RemoteAddr().String()
}

type ClientJSON struct {
	Id   string
	Name string
}

func (c *client) JSONStruct() ClientJSON {
	return ClientJSON{
		Id:   c.id,
		Name: c.name,
	}
}

// Send utils
func (b *Broker) sendRawMessage(conn net.Conn, msg []byte) {
	err := common.SendRawMessage(conn, msg)
	if err != nil {
		b.logger.Errorf("[Net] Could not send message %s to %s: %s", msg, conn, err)
	}
}

// TODO(halfr): take a client instead of a conn
func (b *Broker) sendReply(conn net.Conn, req *cellaserv.Request, data []byte) {
	rep := &cellaserv.Reply{Id: req.Id, Data: data}
	repBytes, err := proto.Marshal(rep)
	if err != nil {
		b.logger.Errorf("[Net] Could not marshal outgoing reply: %s", err)
	}

	msgType := cellaserv.Message_Reply
	msg := &cellaserv.Message{Type: msgType, Content: repBytes}

	common.SendMessage(conn, msg)
}

// TODO(halfr): take a client instead of a conn
func (b *Broker) sendReplyError(conn net.Conn, req *cellaserv.Request, errType cellaserv.Reply_Error_Type) {
	err := &cellaserv.Reply_Error{Type: errType}

	reply := &cellaserv.Reply{Error: err, Id: req.Id}
	replyBytes, _ := proto.Marshal(reply)

	msgType := cellaserv.Message_Reply
	msg := &cellaserv.Message{
		Type:    msgType,
		Content: replyBytes,
	}
	common.SendMessage(conn, msg)
}

// Remove services registered by this connection. The client's mutex must be
// held by caller.
func (b *Broker) removeServicesOnClient(c *client) {
	// TODO: notify goroutines waiting for acks for this service
	for _, s := range c.services {
		b.logger.Infof("[Service] Remove %s", s)
		pubJSON, _ := json.Marshal(s.JSONStruct())
		b.cellaservPublishBytes(logLostService, pubJSON)

		b.servicesMtx.Lock()
		delete(b.services[s.Name], s.Identification)
		b.servicesMtx.Unlock()

		// Close connections that spied this service
		// TODO(halfr): do not close thoses connections, instead,
		// spying and services and make sure that if the service
		// reconnects, the spies are automatically re-added to this
		// service.
		s.spiesMtx.RLock()
		for _, c := range s.spies {
			b.logger.Debugf("[Service] Close spy conn: %s", c)
			if err := c.conn.Close(); err != nil {
				b.logger.Errorf("Could not close connection: %s", err)
			}
		}
		s.spiesMtx.RLock()
	}
}

func (b *Broker) removeSubscriptionsOfClient(c *client) {
	var removedSubscriptions []logSubscriberJSON

	// Remove subscribes from this connection
	removeConnFromMap := func(subMap map[string][]*client) {
		for key, subs := range subMap {
			for i, subClient := range subs {
				if c == subClient {
					// Remove from list of subscribers
					subs[i] = subs[len(subs)-1]
					subMap[key] = subs[:len(subs)-1]

					if len(subMap[key]) == 0 {
						delete(subMap, key)
						break
					}

					removedSubscriptions = append(removedSubscriptions,
						logSubscriberJSON{key, c.conn.RemoteAddr().String()})
				}
			}
		}
	}

	b.subscriberMapMtx.Lock()
	removeConnFromMap(b.subscriberMap)
	b.subscriberMapMtx.Unlock()
	b.subscriberMatchMapMtx.Lock()
	removeConnFromMap(b.subscriberMatchMap)
	b.subscriberMatchMapMtx.Unlock()

	for _, removedSub := range removedSubscriptions {
		pubJSON, _ := json.Marshal(removedSub)
		b.cellaservPublishBytes(logLostSubscriber, pubJSON)
	}

}

func (b *Broker) removeSpiesOnClient(c *client) {
	// Remove conn from the services it spied
	for _, srvc := range c.spying {
		srvc.spiesMtx.Lock()
		for i, spy := range srvc.spies {
			if spy == c {
				// Remove from slice
				srvc.spies[i] = srvc.spies[len(srvc.spies)-1]
				srvc.spies = srvc.spies[:len(srvc.spies)-1]
				break
			}
		}
		srvc.spiesMtx.Unlock()
	}
}

func (b *Broker) newClient(conn net.Conn) *client {
	// Register this connection
	c := &client{
		conn: conn,
		id:   conn.RemoteAddr().String(),
	}
	b.clientsByConn.Store(conn, c)
	b.cellaservPublish(logNewClient, c.JSONStruct())
	return c
}

func (b *Broker) getClientByConn(conn net.Conn) (*client, bool) {
	elt, ok := b.clientsByConn.Load(conn)
	return elt.(*client), ok
}

func (b *Broker) removeClient(c *client) {
	// Client exited, cleaning up resources
	c.mtx.Lock()
	b.removeServicesOnClient(c)
	b.removeSubscriptionsOfClient(c)
	b.removeSpiesOnClient(c)
	c.mtx.Unlock()

	// Remove from list of handled connection
	b.clientsByConn.Delete(c.conn)

	b.cellaservPublish(logLostClient, c.JSONStruct())
}
