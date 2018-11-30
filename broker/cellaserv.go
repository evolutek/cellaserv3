package broker

import (
	"encoding/json"
	"net"

	"bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/evolutek/cellaserv3/common"
	"github.com/golang/protobuf/proto"
)

const (
	// Logs sent by cellaserv
	logCloseConnection = "log.cellaserv.close-connection"
	logConnRename      = "log.cellaserv.connection-rename"
	logLostService     = "log.cellaserv.lost-service"
	logLostSubscriber  = "log.cellaserv.lost-subscriber"
	logNewConnection   = "log.cellaserv.new-connection"
	logNewService      = "log.cellaserv.new-service"
	logNewSubscriber   = "log.cellaserv.new-subscriber"
)

// Send conn data as this struct
type ConnNameJSON struct {
	Addr string
	Name string
}

type SpyRequest struct {
	Service        string
	Identification string
}

// handleDescribeConn attaches a name to the connection that sent the request.
//
// The name of the connection is normally given when a service registers.
// Connections that want to be named too can use this command to do so.
//
// Request payload format: {"name" : string}
func (b *Broker) handleDescribeConn(conn net.Conn, req *cellaserv.Request) {
	var data struct {
		Name string
	}

	if err := json.Unmarshal(req.Data, &data); err != nil {
		b.logger.Warning("[Cellaserv] Could not unmarshal describe-conn: %s, %s", req.Data, err)
		b.sendReplyError(conn, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	b.connNameMap[conn] = data.Name
	newName := b.connDescribe(conn)

	pubJSON, _ := json.Marshal(ConnNameJSON{conn.RemoteAddr().String(), newName})
	b.cellaservPublish(logConnRename, pubJSON)

	b.logger.Debug("[Cellaserv] Describe %s as %s", conn.RemoteAddr(), data.Name)

	b.sendReply(conn, req, nil) // Empty reply
}

func (b *Broker) GetServiceList() []*ServiceJSON {
	// Fix static empty slice that is "null" in JSON
	// A dynamic empty slice is []
	servicesList := make([]*ServiceJSON, 0)
	for _, names := range b.Services {
		for _, s := range names {
			servicesList = append(servicesList, s.JSONStruct())
		}
	}
	return servicesList
}

func (b *Broker) handleListServices(conn net.Conn, req *cellaserv.Request) {
	servicesList := b.GetServiceList()
	data, err := json.Marshal(servicesList)
	if err != nil {
		b.logger.Error("[Cellaserv] Could not marshal the services")
	}
	b.sendReply(conn, req, data)
}

func (b *Broker) GetConnectionList() []ConnNameJSON {
	var conns []ConnNameJSON
	for c := b.connList.Front(); c != nil; c = c.Next() {
		connElt := c.Value.(net.Conn)
		conns = append(conns,
			ConnNameJSON{connElt.RemoteAddr().String(), b.connDescribe(connElt)})
	}
	return conns
}

// handleListConnections replies with the list of currently connected clients
func (b *Broker) handleListConnections(conn net.Conn, req *cellaserv.Request) {
	conns := b.GetConnectionList()
	data, err := json.Marshal(conns)
	if err != nil {
		b.logger.Error("[Cellaserv] Could not marshal the connections list")
	}
	b.sendReply(conn, req, data)
}

type EventInfoJSON map[string][]string

func (b *Broker) GetSubscribersInfo() EventInfoJSON {
	events := make(EventInfoJSON)

	fillMap := func(subMap map[string][]net.Conn) {
		for event, conns := range subMap {
			var connSlice []string
			for _, connItem := range conns {
				connSlice = append(connSlice, connItem.RemoteAddr().String())
			}
			events[event] = connSlice
		}
	}
	fillMap(b.subscriberMap)
	fillMap(b.subscriberMatchMap)

	return events
}

// handleListEvents replies with the list of subscribers
func (b *Broker) handleListEvents(conn net.Conn, req *cellaserv.Request) {
	events := b.GetSubscribersInfo()
	data, err := json.Marshal(events)
	if err != nil {
		b.logger.Error("[Cellaserv] Could not marshal the event list")
	}
	b.sendReply(conn, req, data)
}

// handleShutdown quits cellaserv. Used for debug purposes.
func (b *Broker) handleShutdown() {
	b.logger.Info("[Cellaserv] Shutting down.")

	b.stopProfiling()
	if err := b.mainListener.Close(); err != nil {
		b.logger.Error("Could not close connection: %s", err)
	}
}

func (b *Broker) Shutdown() {
	b.handleShutdown()
}

// handleSpy registers the connection as a spy of a service
func (b *Broker) handleSpy(conn net.Conn, req *cellaserv.Request) {
	var data SpyRequest
	err := json.Unmarshal(req.Data, &data)
	if err != nil {
		b.logger.Warning("[Cellaserv] Could not spy, json error: %s", err)
		b.sendReplyError(conn, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	srvc, ok := b.Services[data.Service][data.Identification]
	if !ok {
		b.logger.Warning("[Cellaserv] Could not spy, no such service: %s %s", data.Service,
			data.Identification)
		b.sendReplyError(conn, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	b.logger.Debug("[Cellaserv] %s spies on %s/%s", b.connDescribe(conn), data.Service,
		data.Identification)

	srvc.Spies = append(srvc.Spies, conn)
	b.connSpies[conn] = append(b.connSpies[conn], srvc)

	b.sendReply(conn, req, nil)
}

// handleVersion return the version of cellaserv
func (b *Broker) handleVersion(conn net.Conn, req *cellaserv.Request) {
	data, err := json.Marshal(common.Version)
	if err != nil {
		b.logger.Warning("[Cellaserv] Could not marshall version, json error: %s", err)
		b.sendReplyError(conn, req, cellaserv.Reply_Error_BadArguments)
		return
	}
	b.sendReply(conn, req, data)
}

// cellaservRequest dispatches requests for cellaserv.
func (b *Broker) cellaservRequest(conn net.Conn, req *cellaserv.Request) {
	switch req.Method {
	case "describe-conn", "describe_conn":
		b.handleDescribeConn(conn, req)
	case "list-connections", "list_connections":
		b.handleListConnections(conn, req)
	case "list-events", "list_events":
		b.handleListEvents(conn, req)
	case "list-services", "list_services":
		b.handleListServices(conn, req)
	case "shutdown":
		b.handleShutdown()
	case "spy":
		b.handleSpy(conn, req)
	case "version":
		b.handleVersion(conn, req)
	default:
		b.sendReplyError(conn, req, cellaserv.Reply_Error_NoSuchMethod)
	}
}

// cellaservPublish sends a publish message from cellaserv
func (b *Broker) cellaservPublish(event string, data []byte) {
	pub := &cellaserv.Publish{Event: event}
	if data != nil {
		pub.Data = data
	}
	pubBytes, err := proto.Marshal(pub)
	if err != nil {
		b.logger.Error("[Cellaserv] Could not marshal event")
		return
	}
	msgType := cellaserv.Message_Publish
	msg := &cellaserv.Message{Type: msgType, Content: pubBytes}
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		b.logger.Error("[Cellaserv] Could not marshal event")
		return
	}

	b.doPublish(msgBytes, pub)
}
