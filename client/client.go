package client

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"bitbucket.org/evolutek/cellaserv3/broker"
	"bitbucket.org/evolutek/cellaserv3/common"
	"github.com/golang/protobuf/proto"
	logging "github.com/op/go-logging"
)

const (
	defaultCellaservPort = "4200"
	defaultCellaservHost = "localhost"
)

type subscriberHandler func(eventName string, eventData []byte)

type subscriber struct {
	eventPattern string
	handle       subscriberHandler
}

type spyHandler func(req *cellaserv.Request, rep *cellaserv.Reply)

type spyPendingRequest struct {
	req   *cellaserv.Request
	spies []spyHandler
}

type client struct {
	logger *logging.Logger

	// Connection to cellaserv
	conn net.Conn
	// Services registered on this client
	services map[string]map[string]*service
	// Subscribers on this client
	subscribers []*subscriber
	// Spies on this client
	spies map[string]map[string][]spyHandler
	// Spy requests missing their associated replies
	spyRequestsPending map[uint64]*spyPendingRequest
	// Nonce used to compute request ids
	currentRequestId uint64
	// Map of request ids to their replies
	requestsInFlight map[uint64]chan *cellaserv.Reply

	msgCh chan *cellaserv.Message
	// TODO(halfr): this should be renamed "serverClosed"
	closeCh chan struct{}
	quit    bool
	quitCh  chan struct{}
}

func (c *client) sendRequestWaitForReply(req *cellaserv.Request) *cellaserv.Reply {
	// Add message Id and increment nonce
	req.Id = atomic.AddUint64(&c.currentRequestId, 1)
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		panic(fmt.Sprintf("Could not marshal request: %s", err))
	}

	if _, ok := c.requestsInFlight[req.Id]; ok {
		panic(fmt.Sprintf("Duplicate Request Id: %d", req.Id))
	}

	// Track request id
	c.requestsInFlight[req.Id] = make(chan *cellaserv.Reply)

	msgType := cellaserv.Message_Request
	msg := cellaserv.Message{Type: msgType, Content: reqBytes}

	common.SendMessage(c.conn, &msg)

	// Wait for reply
	return <-c.requestsInFlight[req.Id]
}

func (c *client) handleRequest(req *cellaserv.Request) error {
	name := req.GetServiceName()
	ident := req.GetServiceIdentification()
	method := req.GetMethod()
	c.logger.Debug("[Request] %s/%s.%s", name, ident, method)

	// Dispatch request to spies
	hasSpied := false
	identsSpied, ok := c.spies[name]
	if ok {
		spies, ok := identsSpied[ident]
		if ok {
			c.logger.Infof("[Spy] Received spied request: %s/%s.%s", name, ident, method)
			hasSpied = true
			// Spy handler is called when the reply to this request is received
			c.spyRequestsPending[req.GetId()] = &spyPendingRequest{
				req:   req,
				spies: spies,
			}
		}
	}

	// Dispatch request to acutal service
	idents, ok := c.services[name]
	if !ok {
		if hasSpied {
			return nil
		}
		return fmt.Errorf("[Request] No such service: %s", name)
	}

	srvc, ok := idents[ident]
	if !ok {
		if hasSpied {
			return nil
		}
		return fmt.Errorf("[Request] No such service identification for %s: %s, has: %v", name, ident, idents)
	}

	replyData, replyErr := srvc.handleRequest(req, method)
	c.sendRequestReply(req, replyData, replyErr)

	return nil
}

func (c *client) sendRequestReply(req *cellaserv.Request, replyData []byte, replyErr error) {
	msgType := cellaserv.Message_Reply
	msgContent := &cellaserv.Reply{Id: req.Id, Data: replyData}

	if replyErr != nil {
		// Log error
		c.logger.Warningf("[Request] Reply error: %s", replyErr)

		// Add error info to reply
		errString := replyErr.Error()
		msgContent.Error = &cellaserv.Reply_Error{
			Type: cellaserv.Reply_Error_Custom,
			What: errString,
		}
	}

	msgContentBytes, _ := proto.Marshal(msgContent)
	msg := &cellaserv.Message{Type: msgType, Content: msgContentBytes}

	common.SendMessage(c.conn, msg)
}

func (c *client) handleReply(rep *cellaserv.Reply) error {
	// Dispatch reply to spies
	hasSpied := false
	spyPending, ok := c.spyRequestsPending[rep.GetId()]
	if ok {
		c.logger.Infof("[Spy] Dispatching request and reply %d", rep.GetId())
		hasSpied = true
		for _, spy := range spyPending.spies {
			spy(spyPending.req, rep)
		}
		// Remove pending request
		delete(c.spyRequestsPending, rep.GetId())
	}

	// Dispatch reply to known requests
	replyChan, ok := c.requestsInFlight[rep.GetId()]
	if !ok {
		if hasSpied {
			return nil
		}
		return fmt.Errorf("Could not find request matching reply: %s", rep.String())
	}
	replyChan <- rep
	return nil
}

func (c *client) handlePublish(pub *cellaserv.Publish) {
	eventName := pub.GetEvent()
	c.logger.Infof("[Publish] Received: %s", eventName)
	for _, h := range c.subscribers {
		if matched, _ := filepath.Match(h.eventPattern, eventName); matched {
			c.logger.Debugf("[Publish] Handling %s", eventName)
			h.handle(eventName, pub.GetData())
		}
	}
}

func (c *client) handleMessage(msg *cellaserv.Message) error {
	var err error

	// Parse and process message payload
	switch msg.Type {
	case cellaserv.Message_Request:
		request := &cellaserv.Request{}
		err = proto.Unmarshal(msg.Content, request)
		if err != nil {
			return fmt.Errorf("Could not unmarshal request: %s", err)
		}
		return c.handleRequest(request)
	case cellaserv.Message_Publish:
		pub := &cellaserv.Publish{}
		err = proto.Unmarshal(msg.Content, pub)
		if err != nil {
			return fmt.Errorf("Could not unmarshal publish: %s", err)
		}
		c.handlePublish(pub)
	case cellaserv.Message_Reply:
		rep := &cellaserv.Reply{}
		err := proto.Unmarshal(msg.Content, rep)
		if err != nil {
			return fmt.Errorf("Could not unmarshal reply: %s", err)
		}
		return c.handleReply(rep)
	case cellaserv.Message_Subscribe:
		fallthrough
	case cellaserv.Message_Register:
		return fmt.Errorf("Client received unsupported message type: %d", msg.Type)
	default:
		return fmt.Errorf("Unknown message type: %d", msg.Type)
	}
	return nil
}

// Close shuts down the client.
func (c *client) Close() {
	c.quit = true
	close(c.quitCh)
}

// Quit returns the receive-only quit channel.
func (c *client) Quit() <-chan struct{} {
	return c.quitCh
}

func (c *client) RegisterService(s *service) {
	// Make sure the second map is created
	if _, ok := c.services[s.Name]; !ok {
		c.services[s.Name] = make(map[string]*service)
	}
	// Keep a pointer to the service
	c.services[s.Name][s.Identification] = s

	// Send register message to cellaserv
	msgType := cellaserv.Message_Register
	msgContent := &cellaserv.Register{
		Name:           s.Name,
		Identification: s.Identification,
	}
	msgContentBytes, _ := proto.Marshal(msgContent)
	msg := &cellaserv.Message{Type: msgType, Content: msgContentBytes}
	common.SendMessage(c.conn, msg)

	c.logger.Infof("Service %s registered", s)
}

func (c *client) Publish(event string, data interface{}) {
	c.logger.Debugf("[Publish] Sending: %s(%v)", event, data)

	// Serialize request payload
	dataBytes, err := json.Marshal(data)
	if err != nil {
		panic(fmt.Sprintf("Could not marshal publish data to JSON: %v", data))
	}

	// Prepare Publish message
	pub := &cellaserv.Publish{
		Event: event,
		Data:  dataBytes,
	}
	pubBytes, err := proto.Marshal(pub)
	if err != nil {
		panic(fmt.Sprintf("Could not marshal publish: %s", err))
	}

	// Send message
	msgType := cellaserv.Message_Publish
	msg := &cellaserv.Message{Type: msgType, Content: pubBytes}
	common.SendMessage(c.conn, msg)
}

func (c *client) Subscribe(eventPattern string, handler subscriberHandler) error {
	// Create and add to subscriber map
	s := &subscriber{
		eventPattern: eventPattern,
		handle:       handler,
	}
	c.logger.Infof("[Subscribe] Subscribing to event pattern: %s", eventPattern)
	c.subscribers = append(c.subscribers, s)

	// Prepare subscribe message
	msgType := cellaserv.Message_Subscribe
	sub := &cellaserv.Subscribe{Event: eventPattern}
	subBytes, err := proto.Marshal(sub)
	if err != nil {
		return fmt.Errorf("Could not marshal subscribe: %s", err)
	}

	msg := cellaserv.Message{Type: msgType, Content: subBytes}

	// Send subscribe message
	common.SendMessage(c.conn, &msg)

	return nil
}

func (c *client) Spy(serviceName string, serviceIdentification string, handler spyHandler) error {
	// Create and add spy handler
	spyIdents, ok := c.spies[serviceName]
	if !ok {
		spyIdents = make(map[string][]spyHandler)
		c.spies[serviceName] = spyIdents
	}
	spyIdents[serviceIdentification] = append(spyIdents[serviceIdentification], handler)

	// Create service stub
	cs := NewServiceStub(c, "cellaserv", "")
	// Make request
	spyArgs := &broker.SpyRequest{
		Service:        serviceName,
		Identification: serviceIdentification,
	}
	cs.Request("spy", spyArgs)

	return nil
}

func newClient(conn net.Conn, name string) *client {
	c := &client{
		logger:             common.NewLogger(name),
		conn:               conn,
		services:           make(map[string]map[string]*service),
		requestsInFlight:   make(map[uint64]chan *cellaserv.Reply),
		spies:              make(map[string]map[string][]spyHandler),
		spyRequestsPending: make(map[uint64]*spyPendingRequest),
		currentRequestId:   rand.Uint64(),
		msgCh:              make(chan *cellaserv.Message),
		closeCh:            make(chan struct{}),
		quitCh:             make(chan struct{}),
	}

	// Receive incoming messages
	go func() {
		for {
			closed, _, msg, err := common.RecvMessage(c.conn)
			if closed {
				close(c.closeCh)
				break
			}
			if err != nil {
				c.logger.Errorf("Could not receive message: %s", err)
				continue
			}
			c.msgCh <- msg
		}
	}()

	// Handle message or quit
	go func() {
	Loop:
		for {
			select {
			case msg := <-c.msgCh:
				err := c.handleMessage(msg)
				if err != nil {
					c.logger.Errorf("[Message] Handle: %s", err)
				}
			case <-c.closeCh:
				if !c.quit {
					close(c.quitCh)
				}
				break Loop
			case <-c.quitCh:
				break Loop
			}
		}
	}()

	return c
}

type ClientOpts struct {
	// Address of the cellaserv server
	CellaservAddr string
	// Name sent to cellaserv to describe the client
	ClientName string
}

// NewConnection returns a Client instance connected to cellaserv or panics
func NewClient(opts ClientOpts) *client {
	// Check cellaserv address
	csAddr := opts.CellaservAddr
	if csAddr == "" {
		csHost := os.Getenv("CS_HOST")
		if csHost == "" {
			csHost = defaultCellaservHost
		}
		csPort := os.Getenv("CS_PORT")
		if csPort == "" {
			csPort = defaultCellaservPort
		}
		csAddr = fmt.Sprintf("%s:%s", csHost, csPort)
	}

	// Connect
	conn, err := net.Dial("tcp", csAddr)
	if err != nil {
		panic(fmt.Errorf("Could not connect to cellaserv: %s", err))
	}

	name := opts.ClientName
	if name == "" {
		name = "client"
	}

	return newClient(conn, name)
}

func init() {
	// Random is used to create message ids
	rand.Seed(time.Now().UnixNano())
}
