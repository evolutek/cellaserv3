package broker

import (
	"encoding/json"
	"net"
	"sync"

	cellaserv "bitbucket.org/evolutek/cellaserv3-protobuf"
	"bitbucket.org/evolutek/cellaserv3/broker/cellaserv/api"
	"bitbucket.org/evolutek/cellaserv3/common"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
)

// client represents a single connnection to cellaserv
type client struct {
	mtx        sync.Mutex    // protects slices below
	conn       net.Conn      // connection of this client
	id         string        // unique id for this client
	name       string        // name of this client
	spying     []*service    // services spied by this client
	services   []*service    // services registered by this clietn
	subscribes []string      // events subscribed by the client
	logger     common.Logger // client logger
}

func (c *client) String() string {
	if c.name != "" {
		return c.name
	}
	return c.conn.RemoteAddr().String()
}

func (c *client) JSONStruct() api.ClientJSON {
	return api.ClientJSON{
		Id:   c.id,
		Name: c.name,
	}
}

func (b *Broker) setClientName(c *client, name string) {
	c.name = name

	// Notify listeners
	b.cellaservPublish(logClientName, c.JSONStruct())
}

// GetClient returns the client struct associated with the client id.
func (b *Broker) GetClient(clientId string) (*client, bool) {
	value, ok := b.mapClientIdToClient.Load(clientId)
	if !ok {
		return nil, false
	}
	return value.(*client), true
}

// Send utils
func (b *Broker) sendRawMessage(conn net.Conn, msg []byte) {
	err := common.SendRawMessage(conn, msg)
	if err != nil {
		b.logger.Errorf("Could not send message %s to %s: %s", msg, conn, err)
	}
}

// TODO(halfr): move from Broker to client
func (b *Broker) sendReply(c *client, req *cellaserv.Request, data []byte) {
	rep := &cellaserv.Reply{Id: req.Id, Data: data}
	repBytes, err := proto.Marshal(rep)
	if err != nil {
		c.logger.Errorf("Could not marshal outgoing reply: %s", err)
	}

	msgType := cellaserv.Message_Reply
	msg := &cellaserv.Message{Type: msgType, Content: repBytes}

	err = common.SendMessage(c.conn, msg)
	if err != nil {
		c.logger.Errorf("Could not send message: %s", err)
	}
}

// TODO(halfr): move from Broker to client
func (b *Broker) sendReplyError(c *client, req *cellaserv.Request, errType cellaserv.Reply_Error_Type) {
	replyErr := &cellaserv.Reply_Error{Type: errType}

	reply := &cellaserv.Reply{Error: replyErr, Id: req.Id}
	replyBytes, _ := proto.Marshal(reply)

	msgType := cellaserv.Message_Reply
	msg := &cellaserv.Message{
		Type:    msgType,
		Content: replyBytes,
	}
	err := common.SendMessage(c.conn, msg)
	if err != nil {
		c.logger.Errorf("Could not send message: %s", err)
	}
}

// Remove services registered by this connection. The client's mutex must be
// held by caller.
func (b *Broker) removeServicesOnClient(c *client) {
	// TODO: notify goroutines waiting for acks for this service
	for _, s := range c.services {
		c.logger.Infof("Remove service %s", s)
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
			c.logger.Debugf("Close spy conn: %s", c)
			if err := c.conn.Close(); err != nil {
				c.logger.Errorf("Could not close connection: %s", err)
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
					removedSubscriptions = append(removedSubscriptions,
						logSubscriberJSON{key, c.id})

					// Remove from list of subscribers
					subs[i] = subs[len(subs)-1]
					subMap[key] = subs[:len(subs)-1]

					if len(subMap[key]) == 0 {
						delete(subMap, key)
						break
					}
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
	id := conn.RemoteAddr().String()
	c := &client{
		conn: conn,
		id:   id,
		logger: log.WithFields(log.Fields{
			"module": "client",
			"client": id,
		}),
	}
	b.mapClientIdToClient.Store(c.id, c)
	b.cellaservPublish(logNewClient, c.JSONStruct())
	return c
}

func (b *Broker) RenameClientFromRequest(req *cellaserv.Request, name string) {
	client, err := b.GetRequestSender(req)
	if err != nil {
		b.logger.Warnf("Could not rename client: %s", err)
		return
	}
	// Set client name
	b.setClientName(client, name)
}

func (b *Broker) removeClient(c *client) {
	// Client exited, cleaning up resources
	c.mtx.Lock()
	b.removeServicesOnClient(c)
	b.removeSubscriptionsOfClient(c)
	b.removeSpiesOnClient(c)
	c.mtx.Unlock()

	// Remove from list of handled connection
	b.mapClientIdToClient.Delete(c.id)

	b.cellaservPublish(logLostClient, c.JSONStruct())
}
