package broker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"path"
	"path/filepath"
	"strings"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"bitbucket.org/evolutek/cellaserv3/common"
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
type ConnectionJSON struct {
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
// TODO(halfr): rename to Name
func (b *Broker) handleDescribeConn(conn net.Conn, req *cellaserv.Request) {
	// TODO(halfr): make this a DescribeConnRequest struct
	var data struct {
		Name string
	}

	if err := json.Unmarshal(req.Data, &data); err != nil {
		b.logger.Warningf("[Cellaserv] Could not unmarshal describe-conn: %s, %s", req.Data, err)
		b.sendReplyError(conn, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	b.logger.Debugf("[Cellaserv] Describe %s as %s", conn.RemoteAddr(), data.Name)

	// Grab internal conn struct
	c, ok := b.getClientByConn(conn)
	if !ok {
		// Connection gone?
		return
	}
	c.name = data.Name

	pubJSON, _ := json.Marshal(ConnectionJSON{
		Addr: conn.RemoteAddr().String(),
		Name: data.Name,
	})
	b.cellaservPublish(logConnRename, pubJSON)

	b.sendReply(conn, req, nil) // Empty reply
}

func (b *Broker) GetServicesJSON() []ServiceJSON {
	// Fix static empty slice that is "null" in JSON
	// A dynamic empty slice is []
	servicesList := make([]ServiceJSON, 0)
	for _, names := range b.services {
		for _, s := range names {
			servicesList = append(servicesList, *s.JSONStruct())
		}
	}
	return servicesList
}

func (b *Broker) handleListServices(conn net.Conn, req *cellaserv.Request) {
	servicesList := b.GetServicesJSON()
	data, err := json.Marshal(servicesList)
	if err != nil {
		b.logger.Errorf("[Cellaserv] Could not marshal the services: %s", err)
	}
	b.sendReply(conn, req, data)
}

func (b *Broker) GetConnectionsJSON() []ConnectionJSON {
	var conns []ConnectionJSON
	b.clientsByConn.Range(func(key, value interface{}) bool {
		c := value.(*client)
		conn := ConnectionJSON{c.conn.RemoteAddr().String(), c.name}
		conns = append(conns, conn)
		return true
	})
	return conns
}

// handleListConnections replies with the list of currently connected clients
func (b *Broker) handleListConnections(conn net.Conn, req *cellaserv.Request) {
	conns := b.GetConnectionsJSON()
	data, err := json.Marshal(conns)
	if err != nil {
		b.logger.Errorf("[Cellaserv] Could not marshal the connections list: %s", err)
	}
	b.sendReply(conn, req, data)
}

type EventsJSON map[string][]string

func (b *Broker) GetEventsJSON() EventsJSON {
	events := make(EventsJSON)

	fillMap := func(subMap map[string][]*client) {
		for event, clients := range subMap {
			var connSlice []string
			for _, c := range clients {
				connSlice = append(connSlice, c.conn.RemoteAddr().String())
			}
			events[event] = connSlice
		}
	}

	b.subscriberMapMtx.RLock()
	fillMap(b.subscriberMap)
	b.subscriberMapMtx.RUnlock()

	b.subscriberMatchMapMtx.RLock()
	fillMap(b.subscriberMatchMap)
	b.subscriberMatchMapMtx.RUnlock()

	return events
}

// handleListEvents replies with the list of subscribers
func (b *Broker) handleListEvents(conn net.Conn, req *cellaserv.Request) {
	events := b.GetEventsJSON()
	data, err := json.Marshal(events)
	if err != nil {
		b.logger.Errorf("[Cellaserv] Could not marshal the event list: %s", err)
	}
	b.sendReply(conn, req, data)
}

// handleShutdown quits cellaserv
func (b *Broker) handleShutdown() {
	b.logger.Info("[Cellaserv] Shutting down.")
	close(b.quitCh)
}

// handleSpy registers the connection as a spy of a service
func (b *Broker) handleSpy(conn net.Conn, req *cellaserv.Request) {
	var data SpyRequest
	err := json.Unmarshal(req.Data, &data)
	if err != nil {
		b.logger.Warningf("[Cellaserv] Could not spy: %s", err)
		b.sendReplyError(conn, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	b.servicesMtx.RLock()
	srvc, ok := b.services[data.Service][data.Identification]
	b.servicesMtx.RUnlock()
	if !ok {
		b.logger.Warningf("[Cellaserv] Could not spy, no such service: %s %s", data.Service,
			data.Identification)
		b.sendReplyError(conn, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	b.logger.Debugf("[Cellaserv] %s spies on %s/%s", b.connDescribe(conn), data.Service,
		data.Identification)

	c, ok := b.getClientByConn(conn)
	if !ok {
		b.logger.Warningf("[Cellaserv]: Client disconnected while processing spy: %s", conn)
		return
	}

	srvc.spiesMtx.Lock()
	srvc.spies = append(srvc.spies, c)
	srvc.spiesMtx.Unlock()

	c.mtx.Lock()
	c.spies = append(c.spies, srvc)
	c.mtx.Unlock()

	b.sendReply(conn, req, nil)
}

// handleVersion return the version of cellaserv
func (b *Broker) handleVersion(conn net.Conn, req *cellaserv.Request) {
	data, err := json.Marshal(common.Version)
	if err != nil {
		b.logger.Warningf("[Cellaserv] Could not marshall version: %s", err)
		b.sendReplyError(conn, req, cellaserv.Reply_Error_BadArguments)
		return
	}
	b.sendReply(conn, req, data)
}

type GetLogsRequest struct {
	Pattern string
}

type GetLogsResponse map[string]string

func (b *Broker) GetLogsByPattern(pattern string) (GetLogsResponse, error) {
	pathPattern := path.Join(b.publishLoggingRoot, pattern)

	if !strings.HasPrefix(pathPattern, path.Join(b.Options.VarRoot, "logs")) {
		err := fmt.Errorf("Don't try to do directory traversal: %s", pattern)
		return nil, err
	}

	// Globbing is allowed
	filenames, err := filepath.Glob(pathPattern)
	if err != nil {
		err := fmt.Errorf("Invalid log globbing: %s, %s", pattern, err)
		return nil, err
	}

	if len(filenames) == 0 {
		err := fmt.Errorf("No such logs: %s", pattern)
		return nil, err
	}

	logs := make(GetLogsResponse)

	for _, filename := range filenames {
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			err := fmt.Errorf("Could not open log: %s: %s", filename, err)
			return nil, err
		}
		logs[path.Base(filename)] = string(data)
	}

	return logs, nil
}

func (b *Broker) handleGetLogs(conn net.Conn, req *cellaserv.Request) {
	var data GetLogsRequest
	err := json.Unmarshal(req.Data, &data)
	if err != nil {
		b.logger.Warningf("[Cellaserv] Invalid get_logs() request: %s", err)
		b.sendReplyError(conn, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	logs, err := b.GetLogsByPattern(data.Pattern)
	if err != nil {
		b.logger.Warningf("[Cellaserv] Could not get logs: %s", err)
		b.sendReplyError(conn, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	logs_json, err := json.Marshal(logs)
	if err != nil {
		b.logger.Warningf("[Cellaserv] Could not serialize response: %s", err)
		b.sendReplyError(conn, req, cellaserv.Reply_Error_BadArguments)
		return
	}
	b.sendReply(conn, req, logs_json)
}

// cellaservRequest dispatches requests for cellaserv.
func (b *Broker) cellaservRequest(conn net.Conn, req *cellaserv.Request) {
	method := strings.Replace(req.Method, "-", "_", -1)
	switch method {
	case "describe_conn":
		b.handleDescribeConn(conn, req)
	case "list_connections":
		b.handleListConnections(conn, req)
	case "list_events":
		b.handleListEvents(conn, req)
	case "list_services":
		b.handleListServices(conn, req)
	case "get_logs":
		b.handleGetLogs(conn, req)
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
		b.logger.Errorf("[Cellaserv] Could not marshal event: %s", err)
		return
	}
	msgType := cellaserv.Message_Publish
	msg := &cellaserv.Message{Type: msgType, Content: pubBytes}
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		b.logger.Errorf("[Cellaserv] Could not marshal event: %s", err)
		return
	}

	b.doPublish(msgBytes, pub)
}
