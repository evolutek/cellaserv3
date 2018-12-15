package broker

import (
	"context"
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
	LogsDir               string
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

// Manage incoming connexion
func (b *Broker) handle(conn net.Conn) {
	c := b.newClient(conn)
	b.logger.Infof("[Broker] New client: %s", c)

	// Handle all messages received on this connection
	for {
		closed, msgBytes, msg, err := common.RecvMessage(conn)
		if err != nil {
			b.logger.Errorf("[Message] Receive: %s", err)
		}
		if closed {
			b.logger.Infof("[Broker] Client disconnected: %s", c)
			break
		}
		err = b.handleMessage(c, msgBytes, msg)
		if err != nil {
			b.logger.Errorf("[Message] Error handling message: %s", err)
		}
	}

	b.removeClient(c)
}

func (b *Broker) logUnmarshalError(msg []byte) {
	dbg := ""
	for _, b := range msg {
		dbg = dbg + fmt.Sprintf("0x%02X ", b)
	}
	b.logger.Errorf("[Broker] Bad message (%d bytes): %s", len(msg), dbg)
}

func (b *Broker) handleMessage(c *client, msgBytes []byte, msg *cellaserv.Message) error {
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
		b.handleRegister(c, register)
		return nil
	case cellaserv.Message_Request:
		request := &cellaserv.Request{}
		err = proto.Unmarshal(msgContent, request)
		if err != nil {
			b.logUnmarshalError(msgContent)
			return fmt.Errorf("Could not unmarshal request: %s", err)
		}
		b.handleRequest(c, msgBytes, request)
		return nil
	case cellaserv.Message_Reply:
		reply := &cellaserv.Reply{}
		err = proto.Unmarshal(msgContent, reply)
		if err != nil {
			b.logUnmarshalError(msgContent)
			return fmt.Errorf("Could not unmarshal reply: %s", err)
		}
		b.handleReply(c, msgBytes, reply)
		return nil
	case cellaserv.Message_Subscribe:
		sub := &cellaserv.Subscribe{}
		err = proto.Unmarshal(msgContent, sub)
		if err != nil {
			b.logUnmarshalError(msgContent)
			return fmt.Errorf("Could not unmarshal subscribe: %s", err)
		}
		b.handleSubscribe(c, sub)
		return nil
	case cellaserv.Message_Publish:
		pub := &cellaserv.Publish{}
		err = proto.Unmarshal(msgContent, pub)
		if err != nil {
			b.logUnmarshalError(msgContent)
			return fmt.Errorf("Could not unmarshal publish: %s", err)
		}
		b.handlePublish(c, msgBytes, pub)
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

	m := &Monitoring{
		Registry: prometheus.NewRegistry(),
		requests: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cellaserv",
			Subsystem: "broker",
			Name:      "request_latency_ms",
			Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 15),
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
