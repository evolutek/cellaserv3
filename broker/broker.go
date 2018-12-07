package broker

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"bitbucket.org/evolutek/cellaserv3/common"
	"github.com/golang/protobuf/proto"
	logging "github.com/op/go-logging"
	"github.com/prometheus/client_golang/prometheus"
)

type Options struct {
	ListenAddress     string
	RequestTimeoutSec time.Duration
	VarRoot           string
	// Rename to PublishLoggingEnabled
	ServiceLoggingEnabled bool
}

type Monitoring struct {
	Registry *prometheus.Registry
	requests *prometheus.HistogramVec
}

type Broker struct {
	Monitoring *Monitoring

	Options *Options

	logger *logging.Logger

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

	// Service logging
	serviceLoggingSession string
	serviceLoggingRoot    string
	serviceLoggingLoggers map[string]*os.File

	// The broker must quit
	quitCh chan struct{}
}

// Manage incoming connexions
func (b *Broker) handle(conn net.Conn) {
	b.logger.Infof("[Broker] Connection opened: %s", b.connDescribe(conn))

	connJSON := connToJSON(conn)
	b.cellaservPublish(logNewConnection, connJSON)

	// Append to list of handled connections
	connListElt := b.connList.PushBack(conn)

	// Handle all messages received on this connection
	for {
		closed, msgBytes, msg, err := common.RecvMessage(conn)
		if err != nil {
			b.logger.Errorf("[Message] Receive: %s", err)
		}
		if closed {
			b.logger.Infof("[Broker] Connection closed: %s", b.connDescribe(conn))
			break
		}
		err = b.handleMessage(conn, msgBytes, msg)
		if err != nil {
			b.logger.Errorf("[Message] Handle: %s", err)
		}
	}

	// Remove from list of handled connection
	b.connList.Remove(connListElt)

	// Clean connection name, if not given this is a noop
	delete(b.connNameMap, conn)

	// Remove services registered by this connection
	// TODO: notify goroutines waiting for acks for this service
	for _, s := range b.servicesConn[conn] {
		b.logger.Infof("[Service] Remove %s", s)
		pubJSON, _ := json.Marshal(s.JSONStruct())
		b.cellaservPublish(logLostService, pubJSON)
		delete(b.services[s.Name], s.Identification)

		// Close connections that spied this service
		for _, c := range s.Spies {
			b.logger.Debugf("[Service] Close spy conn: %s", b.connDescribe(c))
			if err := c.Close(); err != nil {
				b.logger.Errorf("Could not close connection: %s", err)
			}
		}
	}
	delete(b.servicesConn, conn)

	// Remove subscribes from this connection
	removeConnFromMap := func(subMap map[string][]net.Conn) {
		for key, subs := range subMap {
			for i, subConn := range subs {
				if conn == subConn {
					// Remove from list of subscribers
					subs[i] = subs[len(subs)-1]
					subMap[key] = subs[:len(subs)-1]

					pubJSON, _ := json.Marshal(
						logSubscriberJSON{key, b.connDescribe(conn)})
					b.cellaservPublish(logLostSubscriber, pubJSON)

					if len(subMap[key]) == 0 {
						delete(subMap, key)
						break
					}
				}
			}
		}
	}
	removeConnFromMap(b.subscriberMap)
	removeConnFromMap(b.subscriberMatchMap)

	// Remove conn from the services it spied
	for _, srvc := range b.connSpies[conn] {
		for i, connItem := range srvc.Spies {
			if connItem == conn {
				// Remove from slice
				srvc.Spies[i] = srvc.Spies[len(srvc.Spies)-1]
				srvc.Spies = srvc.Spies[:len(srvc.Spies)-1]
				break
			}
		}
	}
	delete(b.connSpies, conn)

	b.cellaservPublish(logCloseConnection, connJSON)
}

func (b *Broker) logUnmarshalError(msg []byte) {
	dbg := ""
	for _, b := range msg {
		dbg = dbg + fmt.Sprintf("0x%02X ", b)
	}
	b.logger.Errorf("[Broker] Bad message (%d bytes): %s", len(msg), dbg)
}

func (b *Broker) handleMessage(conn net.Conn, msgBytes []byte, msg *cellaserv.Message) error {
	var err error

	// Parse and process message payload
	msgContent := msg.GetContent()

	switch msg.GetType() {
	case cellaserv.Message_Register:
		register := &cellaserv.Register{}
		err = proto.Unmarshal(msgContent, register)
		if err != nil {
			b.logUnmarshalError(msgContent)
			return fmt.Errorf("Could not unmarshal register: %s", err)
		}
		b.handleRegister(conn, register)
		return nil
	case cellaserv.Message_Request:
		request := &cellaserv.Request{}
		err = proto.Unmarshal(msgContent, request)
		if err != nil {
			b.logUnmarshalError(msgContent)
			return fmt.Errorf("Could not unmarshal request: %s", err)
		}
		b.handleRequest(conn, msgBytes, request)
		return nil
	case cellaserv.Message_Reply:
		reply := &cellaserv.Reply{}
		err = proto.Unmarshal(msgContent, reply)
		if err != nil {
			b.logUnmarshalError(msgContent)
			return fmt.Errorf("Could not unmarshal reply: %s", err)
		}
		b.handleReply(conn, msgBytes, reply)
		return nil
	case cellaserv.Message_Subscribe:
		sub := &cellaserv.Subscribe{}
		err = proto.Unmarshal(msgContent, sub)
		if err != nil {
			b.logUnmarshalError(msgContent)
			return fmt.Errorf("Could not unmarshal subscribe: %s", err)
		}
		b.handleSubscribe(conn, sub)
		return nil
	case cellaserv.Message_Publish:
		pub := &cellaserv.Publish{}
		err = proto.Unmarshal(msgContent, pub)
		if err != nil {
			b.logUnmarshalError(msgContent)
			return fmt.Errorf("Could not unmarshal publish: %s", err)
		}
		b.handlePublish(conn, msgBytes, pub)
		return nil
	default:
		return fmt.Errorf("Unknown message type: %d", msg.Type)
	}
}

// Handles incoming connections
func (b *Broker) serve(l net.Listener) error {
	for {
		conn, err := l.Accept()
		nerr, ok := err.(net.Error)
		if ok {
			if nerr.Temporary() {
				b.logger.Warningf("[Broker] Could not accept: %s", err)
				time.Sleep(10 * time.Millisecond)
				continue
			} else {
				return err
			}
		}
		go b.handle(conn)
	}
	return nil
}

func (b *Broker) Run(ctx context.Context) error {
	if b.Options.ServiceLoggingEnabled {
		err := b.rotateServiceLogs()
		if err != nil {
			return err
		}
	}

	// Create TCP listenener for incoming connections
	l, err := net.Listen("tcp", b.Options.ListenAddress)
	if err != nil {
		b.logger.Errorf("[Broker] Could not listen: %s", err)
		return err
	}
	defer l.Close()

	b.logger.Infof("[Broker] Listening on %s", b.Options.ListenAddress)

	errCh := make(chan error)
	go func() {
		errCh <- b.serve(l)
	}()

	select {
	case e := <-errCh:
		return e
	case <-b.quitCh:
		return nil
	case <-ctx.Done():
		return nil
	}
}

// TODO(halfr): do not use a pointer for options
func New(options *Options, logger *logging.Logger) *Broker {
	// Set default options
	if options.RequestTimeoutSec == 0 {
		options.RequestTimeoutSec = 5
	}

	m := &Monitoring{
		Registry: prometheus.NewRegistry(),
		requests: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cellaserv",
			Subsystem: "broker",
			Name:      "requests",
		}, []string{"service", "identification", "method"}),
	}

	broker := &Broker{
		Options: options,
		logger:  logger,

		Monitoring: m,

		connNameMap:        make(map[net.Conn]string),
		connSpies:          make(map[net.Conn][]*service),
		services:           make(map[string]map[string]*service),
		servicesConn:       make(map[net.Conn][]*service),
		reqIds:             make(map[uint64]*requestTracking),
		subscriberMap:      make(map[string][]net.Conn),
		subscriberMatchMap: make(map[string][]net.Conn),
		connList:           list.New(),

		quitCh: make(chan struct{}),
	}

	// Setup monitoring
	m.Registry.MustRegister(m.requests)
	m.Registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "cellaserv",
		Subsystem: "broker",
		Name:      "connections",
	}, func() float64 { return float64(broker.connList.Len()) }))
	m.Registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "cellaserv",
		Subsystem: "broker",
		Name:      "requests_pending",
	}, func() float64 { return float64(len(broker.reqIds)) }))

	return broker
}
