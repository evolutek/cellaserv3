package broker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"bitbucket.org/evolutek/cellaserv3/common"
	"github.com/golang/protobuf/proto"
)

const (
	logClientName     = "log.cellaserv.client-name"
	logLostClient     = "log.cellaserv.lost-client"
	logLostService    = "log.cellaserv.lost-service"
	logLostSubscriber = "log.cellaserv.lost-subscriber"
	logNewClient      = "log.cellaserv.new-client"
	logNewService     = "log.cellaserv.new-service"
	logNewSubscriber  = "log.cellaserv.new-subscriber"
)

type NameClientRequest struct {
	Name string
}

// handleNameClient attaches a name to the client that sent the request.
//
// The name of the client is normally given when a service registers.
// Connections that want to be named too can use this command to do so.
func (b *Broker) handleNameClient(c *client, req *cellaserv.Request) {
	var data NameClientRequest
	if err := json.Unmarshal(req.Data, &data); err != nil {
		b.logger.Warningf("[Cellaserv] Could not unmarshal request data: %s, %s", req.Data, err)
		b.sendReplyError(c, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	b.logger.Infof("[Cellaserv] Name %s as %s", c, data.Name)

	// Sets client's name
	c.name = data.Name

	b.cellaservPublish(logClientName, c.JSONStruct())

	b.sendReply(c, req, nil) // Empty reply
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

func (b *Broker) handleListServices(c *client, req *cellaserv.Request) {
	servicesList := b.GetServicesJSON()
	data, err := json.Marshal(servicesList)
	if err != nil {
		b.logger.Errorf("[Cellaserv] Could not marshal the services: %s", err)
	}
	b.sendReply(c, req, data)
}

func (b *Broker) GetClientsJSON() []ClientJSON {
	var conns []ClientJSON
	b.clientsByConn.Range(func(key, value interface{}) bool {
		c := value.(*client)
		conn := c.JSONStruct()
		conns = append(conns, conn)
		return true
	})
	return conns
}

// handleListClients replies with the list of currently connected clients
func (b *Broker) handleListClients(c *client, req *cellaserv.Request) {
	conns := b.GetClientsJSON()
	data, err := json.Marshal(conns)
	if err != nil {
		b.logger.Errorf("[Cellaserv] Could not marshal the clients list: %s", err)
	}
	b.sendReply(c, req, data)
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
func (b *Broker) handleListEvents(c *client, req *cellaserv.Request) {
	events := b.GetEventsJSON()
	data, err := json.Marshal(events)
	if err != nil {
		b.logger.Errorf("[Cellaserv] Could not marshal the event list: %s", err)
	}
	b.sendReply(c, req, data)
}

// handleShutdown quits cellaserv
func (b *Broker) handleShutdown() {
	b.logger.Info("[Cellaserv] Shutting down.")
	close(b.quitCh)
}

type SpyRequest struct {
	Service        string
	Identification string
}

// handleSpy registers the connection as a spy of a service
func (b *Broker) handleSpy(c *client, req *cellaserv.Request) {
	var data SpyRequest
	err := json.Unmarshal(req.Data, &data)
	if err != nil {
		b.logger.Warningf("[Cellaserv] Could not spy: %s", err)
		b.sendReplyError(c, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	b.servicesMtx.RLock()
	srvc, ok := b.services[data.Service][data.Identification]
	b.servicesMtx.RUnlock()
	if !ok {
		b.logger.Warningf("[Cellaserv] Could not spy, no such service: %s %s", data.Service,
			data.Identification)
		b.sendReplyError(c, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	b.logger.Debugf("[Cellaserv] %s spies on %s[%s]", c, data.Service,
		data.Identification)

	srvc.spiesMtx.Lock()
	srvc.spies = append(srvc.spies, c)
	srvc.spiesMtx.Unlock()

	c.mtx.Lock()
	c.spying = append(c.spying, srvc)
	c.mtx.Unlock()

	b.sendReply(c, req, nil)
}

// handleVersion return the version of cellaserv
func (b *Broker) handleVersion(c *client, req *cellaserv.Request) {
	data, err := json.Marshal(common.Version)
	if err != nil {
		b.logger.Warningf("[Cellaserv] Could not marshal version: %s", err)
		b.sendReplyError(c, req, cellaserv.Reply_Error_BadArguments)
		return
	}
	b.sendReply(c, req, data)
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

func (b *Broker) handleGetLogs(c *client, req *cellaserv.Request) {
	var data GetLogsRequest
	err := json.Unmarshal(req.Data, &data)
	if err != nil {
		b.logger.Warningf("[Cellaserv] Invalid get_logs() request: %s", err)
		b.sendReplyError(c, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	logs, err := b.GetLogsByPattern(data.Pattern)
	if err != nil {
		b.logger.Warningf("[Cellaserv] Could not get logs: %s", err)
		b.sendReplyError(c, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	logs_json, err := json.Marshal(logs)
	if err != nil {
		b.logger.Warningf("[Cellaserv] Could not serialize response: %s", err)
		b.sendReplyError(c, req, cellaserv.Reply_Error_BadArguments)
		return
	}
	b.sendReply(c, req, logs_json)
}

// cellaservRequest dispatches requests for cellaserv.
func (b *Broker) cellaservRequest(c *client, req *cellaserv.Request) {
	method := strings.Replace(req.Method, "-", "_", -1)
	switch method {
	case "describe_conn":
		b.handleNameClient(c, req)
	case "list_clients":
		b.handleListClients(c, req)
	case "list_events":
		b.handleListEvents(c, req)
	case "list_services":
		b.handleListServices(c, req)
	case "get_logs":
		b.handleGetLogs(c, req)
	case "shutdown":
		b.handleShutdown()
	case "spy":
		b.handleSpy(c, req)
	case "version":
		b.handleVersion(c, req)
	default:
		b.sendReplyError(c, req, cellaserv.Reply_Error_NoSuchMethod)
	}
}

// cellaservPublishBytes sends a publish message from cellaserv
func (b *Broker) cellaservPublishBytes(event string, data []byte) {
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

func (b *Broker) cellaservPublish(event string, obj interface{}) {
	pubData, err := json.Marshal(obj)
	if err != nil {
		b.logger.Errorf("Unable to marshal publish: %s", err)
		return
	}
	b.cellaservPublishBytes(event, pubData)
}
