package broker

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/evolutek/cellaserv3/common"
	"github.com/golang/protobuf/proto"
	logging "gopkg.in/op/go-logging.v1"
)

var (
	// Main logger
	log *logging.Logger

	// Socket where all incoming connections go
	mainListener net.Listener

	// List of all currently handled connections
	connList *list.List

	// Map a connection to a name, filled with cellaserv.descrbie-conn
	connNameMap map[net.Conn]string

	// Map a connection to the service it spies
	connSpies map[net.Conn][]*service

	// Map of currently connected services by name, then identification
	services map[string]map[string]*service

	// Map of all services associated with a connection
	servicesConn map[net.Conn][]*service

	// Map of requests ids with associated timeout timer
	reqIds             map[uint64]*requestTracking
	subscriberMap      map[string][]net.Conn
	subscriberMatchMap map[string][]net.Conn
)

type Options struct {
	ListenAddress string
}

type Broker struct {
	logger *logging.Logger

	Options *Options

	// Map of currently connected services by name, then identification
	Services map[string]map[string]*service
}

// Manage incoming connexions
func handle(conn net.Conn) {
	log.Info("[Net] Connection opened: %s", connDescribe(conn))

	connJSON := connToJSON(conn)
	cellaservPublish(logNewConnection, connJSON)

	// Append to list of handled connections
	connListElt := connList.PushBack(conn)

	// Handle all messages received on this connection
	for {
		closed, msgBytes, msg, err := common.RecvMessage(conn)
		if err != nil {
			log.Error("[Message] Receive: %s", err)
		}
		if closed {
			log.Info("[Net] Connection closed: %s", connDescribe(conn))
			break
		}
		err = handleMessage(conn, msgBytes, msg)
		if err != nil {
			log.Error("[Message] Handle: %s", err)
		}
	}

	// Remove from list of handled connection
	connList.Remove(connListElt)

	// Clean connection name, if not given this is a noop
	delete(connNameMap, conn)

	// Remove services registered by this connection
	// TODO: notify goroutines waiting for acks for this service
	for _, s := range servicesConn[conn] {
		log.Info("[Services] Remove %s", s)
		pubJSON, _ := json.Marshal(s.JSONStruct())
		cellaservPublish(logLostService, pubJSON)
		delete(services[s.Name], s.Identification)

		// Close connections that spied this service
		for _, c := range s.Spies {
			log.Debug("[Service] Close spy conn: %s", connDescribe(c))
			if err := c.Close(); err != nil {
				log.Error("Could not close connection:", err)
			}
		}
	}
	delete(servicesConn, conn)

	// Remove subscribes from this connection
	removeConnFromMap := func(subMap map[string][]net.Conn) {
		for key, subs := range subMap {
			for i, subConn := range subs {
				if conn == subConn {
					// Remove from list of subscribers
					subs[i] = subs[len(subs)-1]
					subMap[key] = subs[:len(subs)-1]

					pubJSON, _ := json.Marshal(
						logSubscriberJSON{key, connDescribe(conn)})
					cellaservPublish(logLostSubscriber, pubJSON)

					if len(subMap[key]) == 0 {
						delete(subMap, key)
						break
					}
				}
			}
		}
	}
	removeConnFromMap(subscriberMap)
	removeConnFromMap(subscriberMatchMap)

	// Remove conn from the services it spied
	for _, srvc := range connSpies[conn] {
		for i, connItem := range srvc.Spies {
			if connItem == conn {
				// Remove from slice
				srvc.Spies[i] = srvc.Spies[len(srvc.Spies)-1]
				srvc.Spies = srvc.Spies[:len(srvc.Spies)-1]
				break
			}
		}
	}
	delete(connSpies, conn)

	cellaservPublish(logCloseConnection, connJSON)
}

func logUnmarshalError(msg []byte) {
	dbg := ""
	for _, b := range msg {
		dbg = dbg + fmt.Sprintf("0x%02X ", b)
	}
	log.Error("[Net] Bad message (%d bytes): %s", len(msg), dbg)
}

func handleMessage(conn net.Conn, msgBytes []byte, msg *cellaserv.Message) error {
	var err error

	// Parse and process message payload
	msgContent := msg.GetContent()

	switch msg.GetType() {
	case cellaserv.Message_Register:
		register := &cellaserv.Register{}
		err = proto.Unmarshal(msgContent, register)
		if err != nil {
			logUnmarshalError(msgContent)
			return fmt.Errorf("Could not unmarshal register: %s", err)
		}
		handleRegister(conn, register)
		return nil
	case cellaserv.Message_Request:
		request := &cellaserv.Request{}
		err = proto.Unmarshal(msgContent, request)
		if err != nil {
			logUnmarshalError(msgContent)
			return fmt.Errorf("Could not unmarshal request: %s", err)
		}
		handleRequest(conn, msgBytes, request)
		return nil
	case cellaserv.Message_Reply:
		reply := &cellaserv.Reply{}
		err = proto.Unmarshal(msgContent, reply)
		if err != nil {
			logUnmarshalError(msgContent)
			return fmt.Errorf("Could not unmarshal reply: %s", err)
		}
		handleReply(conn, msgBytes, reply)
		return nil
	case cellaserv.Message_Subscribe:
		sub := &cellaserv.Subscribe{}
		err = proto.Unmarshal(msgContent, sub)
		if err != nil {
			logUnmarshalError(msgContent)
			return fmt.Errorf("Could not unmarshal subscribe: %s", err)
		}
		handleSubscribe(conn, sub)
		return nil
	case cellaserv.Message_Publish:
		pub := &cellaserv.Publish{}
		err = proto.Unmarshal(msgContent, pub)
		if err != nil {
			logUnmarshalError(msgContent)
			return fmt.Errorf("Could not unmarshal publish: %s", err)
		}
		handlePublish(conn, msgBytes, pub)
		return nil
	default:
		return fmt.Errorf("Unknown message type: %d", *msg.Type)
	}
}

func setup() {
	// Get main logger
	log = common.GetLog()

	// Initialize our maps
	connNameMap = make(map[net.Conn]string)
	connSpies = make(map[net.Conn][]*service)
	services = make(map[string]map[string]*service)
	servicesConn = make(map[net.Conn][]*service)
	reqIds = make(map[uint64]*requestTracking)
	subscriberMap = make(map[string][]net.Conn)
	subscriberMatchMap = make(map[string][]net.Conn)
	connList = list.New()

	// Configure CPU profiling, stopped when cellaserv receive the kill request
	setupProfiling()
}

// ListenAndServe starts the cellaserv broker
func ListenAndServe(sockAddrListen string) error {
	setup()

	// Create TCP listenener for incoming connections
	var err error
	mainListener, err = net.Listen("tcp", sockAddrListen)
	if err != nil {
		log.Error("[Net] Could not listen: %s", err)
		return err
	}

	log.Info("[Net] Listening on %s", sockAddrListen)

	// Handle new connections
	for {
		conn, err := mainListener.Accept()
		nerr, ok := err.(net.Error)
		if ok {
			if nerr.Temporary() {
				log.Warning("[Net] Could not accept: %s", err)
				time.Sleep(10 * time.Millisecond)
				continue
			} else {
				log.Error("[Net] Connection unavailable: %s", err)
				break
			}
		}

		go handle(conn)
	}

	return nil
}

func (b *Broker) Run(ctx context.Context) error {
	return ListenAndServe(b.Options.ListenAddress)
}

func New(logger *logging.Logger, options *Options) *Broker {
	return &Broker{
		logger:  logger,
		Options: options,
	}
}
