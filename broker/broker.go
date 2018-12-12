package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	_ "net/http/pprof"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"bitbucket.org/evolutek/cellaserv3/common"
	"github.com/golang/protobuf/proto"
	logging "github.com/op/go-logging"
	"github.com/prometheus/client_golang/prometheus"
)

type Options struct {
	ListenAddress         string
	RequestTimeoutSec     time.Duration
	VarRoot               string
	PublishLoggingEnabled bool
}

type Monitoring struct {
	Registry *prometheus.Registry
	requests *prometheus.HistogramVec
}

type Broker struct {
	Monitoring *Monitoring

	Options *Options

	logger *logging.Logger

	// All currently handled connections
	clientsByConn sync.Map // map[net.Conn]*client

	// Map of currently connected services by name, then identification
	servicesMtx sync.RWMutex
	services    map[string]map[string]*service

	// Map of requests ids with associated timeout timer
	reqIdsMtx sync.RWMutex
	reqIds    map[uint64]*requestTracking

	// Subscriber management
	subscriberMapMtx      sync.RWMutex
	subscriberMap         map[string][]*client
	subscriberMatchMapMtx sync.RWMutex
	subscriberMatchMap    map[string][]*client

	// Publish logging
	publishLoggingSession string
	publishLoggingRoot    string
	publishLoggingLoggers sync.Map // map[string]*os.File

	// The broker is started
	startedCh chan struct{}
	// The broker must quit
	quitCh chan struct{}
}

func (b *Broker) getClientByConn(conn net.Conn) (*client, bool) {
	elt, ok := b.clientsByConn.Load(conn)
	return elt.(*client), ok
}

// Remove services registered by this connection. The client's mutex must be
// held by caller.
func (b *Broker) removeServicesOnClient(c *client) {
	// TODO: notify goroutines waiting for acks for this service
	for _, s := range c.services {
		b.logger.Infof("[Service] Remove %s", s)
		pubJSON, _ := json.Marshal(s.JSONStruct())
		b.cellaservPublish(logLostService, pubJSON)

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
		b.cellaservPublish(logLostSubscriber, pubJSON)
	}

}

func (b *Broker) removeSpiesOnClient(c *client) {
	// Remove conn from the services it spied
	for _, srvc := range c.spies {
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

// Manage incoming connexion
func (b *Broker) handle(conn net.Conn) {
	b.logger.Infof("[Broker] Connection opened: %s", b.connDescribe(conn))

	// Register this connection
	c := &client{conn: conn}
	b.clientsByConn.Store(conn, c)

	// TODO(halfr): use c.ToJSON()
	connJSON := connToJSON(conn)
	b.cellaservPublish(logNewConnection, connJSON)

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

	// Client exited, cleaning up resources
	c.mtx.Lock()
	b.removeServicesOnClient(c)
	b.removeSubscriptionsOfClient(c)
	b.removeSpiesOnClient(c)
	c.mtx.Unlock()

	// Remove from list of handled connection
	b.clientsByConn.Delete(conn)

	// Publish that the client disconnected
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
func (b *Broker) serve(l net.Listener, errCh chan error) {
	b.logger.Infof("[Broker] Listening on %s", b.Options.ListenAddress)

	for {
		conn, err := l.Accept()
		nerr, ok := err.(net.Error)
		if ok {
			if nerr.Temporary() {
				b.logger.Warningf("[Broker] Could not accept: %s", err)
				time.Sleep(10 * time.Millisecond)
				continue
			} else {
				errCh <- err
				break
			}
		}
		go b.handle(conn)
	}
}

func (b *Broker) quit() chan struct{} {
	return b.quitCh
}

func (b *Broker) Run(ctx context.Context) error {
	if b.Options.PublishLoggingEnabled {
		err := b.rotatePublishLoggers()
		if err != nil {
			return err
		}
	}

	errCh := make(chan error)

	// Create TCP listenener for incoming connections
	l, err := net.Listen("tcp", b.Options.ListenAddress)
	if err != nil {
		b.logger.Errorf("[Broker] Could not listen: %s", err)
		errCh <- err
	} else {
		defer l.Close()
	}

	go b.serve(l, errCh)

	close(b.startedCh)

	select {
	case e := <-errCh:
		return e
	case <-b.quit():
		return nil
	case <-ctx.Done():
		return nil
	}
}

func New(options Options, logger *logging.Logger) *Broker {
	// Set default options
	if options.RequestTimeoutSec == 0 {
		options.RequestTimeoutSec = 5
	}

	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	m := &Monitoring{
		Registry: prometheus.NewRegistry(),
		requests: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cellaserv",
			Subsystem: "broker",
			Name:      "requests",
		}, []string{"service", "identification", "method"}),
	}

	broker := &Broker{
		Options: &options,
		logger:  logger,

		Monitoring: m,

		services:           make(map[string]map[string]*service),
		reqIds:             make(map[uint64]*requestTracking),
		subscriberMap:      make(map[string][]*client),
		subscriberMatchMap: make(map[string][]*client),

		startedCh: make(chan struct{}),
		quitCh:    make(chan struct{}),
	}

	// Setup monitoring
	m.Registry.MustRegister(m.requests)
	m.Registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "cellaserv",
		Subsystem: "broker",
		Name:      "requests_pending",
	}, func() float64 { return float64(len(broker.reqIds)) }))

	return broker
}
