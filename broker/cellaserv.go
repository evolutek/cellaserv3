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
type connNameJSON struct {
	Addr string
	Name string
}

// handleDescribeConn attaches a name to the connection that sent the request.
//
// The name of the connection is normally given when a service registers.
// Connections that want to be named too can use this command to do so.
//
// Request payload format: {"name" : string}
func handleDescribeConn(conn net.Conn, req *cellaserv.Request) {
	var data struct {
		Name string
	}

	if err := json.Unmarshal(req.Data, &data); err != nil {
		log.Warning("[Cellaserv] Could not unmarshal describe-conn: %s, %s", req.Data, err)
		sendReplyError(conn, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	connNameMap[conn] = data.Name
	newName := connDescribe(conn)

	pubJSON, _ := json.Marshal(connNameJSON{conn.RemoteAddr().String(), newName})
	cellaservPublish(logConnRename, pubJSON)

	log.Debug("[Cellaserv] Describe %s as %s", conn.RemoteAddr(), data.Name)

	sendReply(conn, req, nil) // Empty reply
}

func handleListServices(conn net.Conn, req *cellaserv.Request) {
	// Fix static empty slice that is "null" in JSON
	// A dynamic empty slice is []
	servicesList := make([]*ServiceJSON, 0)
	for _, names := range services {
		for _, s := range names {
			servicesList = append(servicesList, s.JSONStruct())
		}
	}

	data, err := json.Marshal(servicesList)
	if err != nil {
		log.Error("[Cellaserv] Could not marshal the services")
	}
	sendReply(conn, req, data)
}

// handleListConnections replies with the list of currently connected clients
func handleListConnections(conn net.Conn, req *cellaserv.Request) {
	var conns []connNameJSON
	for c := connList.Front(); c != nil; c = c.Next() {
		connElt := c.Value.(net.Conn)
		conns = append(conns,
			connNameJSON{connElt.RemoteAddr().String(), connDescribe(connElt)})
	}

	data, err := json.Marshal(conns)
	if err != nil {
		log.Error("[Cellaserv] Could not marshal the connections list")
	}
	sendReply(conn, req, data)
}

// handleListEvents replies with the list of subscribers
func handleListEvents(conn net.Conn, req *cellaserv.Request) {
	events := make(map[string][]string)

	fillMap := func(subMap map[string][]net.Conn) {
		for event, conns := range subMap {
			var connSlice []string
			for _, connItem := range conns {
				connSlice = append(connSlice, connItem.RemoteAddr().String())
			}
			events[event] = connSlice
		}
	}

	fillMap(subscriberMap)
	fillMap(subscriberMatchMap)

	data, err := json.Marshal(events)
	if err != nil {
		log.Error("[Cellaserv] Could not marshal the event list")
	}
	sendReply(conn, req, data)
}

// handleShutdown quits cellaserv. Used for debug purposes.
func handleShutdown() {
	log.Info("[Cellaserv] Shutting down.")

	stopProfiling()
	if err := mainListener.Close(); err != nil {
		log.Error("Could not close connection: %s", err)
	}
}

func Shutdown() {
	handleShutdown()
}

// handleSpy registers the connection as a spy of a service
func handleSpy(conn net.Conn, req *cellaserv.Request) {
	var data struct {
		Service        string
		Identification string
	}

	err := json.Unmarshal(req.Data, &data)
	if err != nil {
		log.Warning("[Cellaserv] Could not spy, json error: %s", err)
		sendReplyError(conn, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	srvc, ok := services[data.Service][data.Identification]
	if !ok {
		log.Warning("[Cellaserv] Could not spy, no such service: %s %s", data.Service,
			data.Identification)
		sendReplyError(conn, req, cellaserv.Reply_Error_BadArguments)
		return
	}

	log.Debug("[Cellaserv] %s spies on %s/%s", connDescribe(conn), data.Service,
		data.Identification)

	srvc.Spies = append(srvc.Spies, conn)
	connSpies[conn] = append(connSpies[conn], srvc)

	sendReply(conn, req, nil)
}

// handleVersion return the version of cellaserv
func handleVersion(conn net.Conn, req *cellaserv.Request) {
	data, err := json.Marshal(common.Version)
	if err != nil {
		log.Warning("[Cellaserv] Could not marshall version, json error: %s", err)
		sendReplyError(conn, req, cellaserv.Reply_Error_BadArguments)
		return
	}
	sendReply(conn, req, data)
}

// cellaservRequest dispatches requests for cellaserv.
func cellaservRequest(conn net.Conn, req *cellaserv.Request) {
	switch *req.Method {
	case "describe-conn", "describe_conn":
		handleDescribeConn(conn, req)
	case "list-connections", "list_connections":
		handleListConnections(conn, req)
	case "list-events", "list_events":
		handleListEvents(conn, req)
	case "list-services", "list_services":
		handleListServices(conn, req)
	case "shutdown":
		handleShutdown()
	case "spy":
		handleSpy(conn, req)
	case "version":
		handleVersion(conn, req)
	default:
		sendReplyError(conn, req, cellaserv.Reply_Error_NoSuchMethod)
	}
}

// cellaservPublish sends a publish message from cellaserv
func cellaservPublish(event string, data []byte) {
	pub := &cellaserv.Publish{Event: &event}
	if data != nil {
		pub.Data = data
	}
	pubBytes, err := proto.Marshal(pub)
	if err != nil {
		log.Error("[Cellaserv] Could not marshal event")
		return
	}
	msgType := cellaserv.Message_Publish
	msg := &cellaserv.Message{Type: &msgType, Content: pubBytes}
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		log.Error("[Cellaserv] Could not marshal event")
		return
	}

	doPublish(msgBytes, pub)
}
