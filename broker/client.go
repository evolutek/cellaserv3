package broker

import (
	"encoding/json"
	"net"
	"sync"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"bitbucket.org/evolutek/cellaserv3/common"
	"github.com/golang/protobuf/proto"
)

// client represents a single connnection to cellaserv
type client struct {
	mtx  sync.Mutex // protects slices below
	conn net.Conn   // connection of this client
	name string     // name of this client
	// TODO(halfr): rename to "spying"
	spies    []*service // services spied by this client
	services []*service // services registered by this clietn
}

// TODO(halfr): remove and use client
type connJSON struct {
	Addr string
}

func (c *client) String() string {
	if c.name != "" {
		return c.name
	}
	return c.conn.RemoteAddr().String()
}

func connToJSON(conn net.Conn) []byte {
	ret, _ := json.Marshal(connJSON{conn.RemoteAddr().String()})
	return ret
}

// connDesribe returns all the information cellaserv have on the connection
// Deprecated: get the client and call client.String()
func (b *Broker) connDescribe(conn net.Conn) string {
	return conn.RemoteAddr().String()
}

// Send utils
func (b *Broker) sendRawMessage(conn net.Conn, msg []byte) {
	err := common.SendRawMessage(conn, msg)
	if err != nil {
		b.logger.Errorf("[Net] Could not send message %s to %s: %s", msg, conn, err)
	}
}

func (b *Broker) sendReply(conn net.Conn, req *cellaserv.Request, data []byte) {
	rep := &cellaserv.Reply{Id: req.Id, Data: data}
	repBytes, err := proto.Marshal(rep)
	if err != nil {
		b.logger.Errorf("[Net] Could not marshal outgoing reply: %s", err)
	}

	msgType := cellaserv.Message_Reply
	msg := &cellaserv.Message{Type: msgType, Content: repBytes}

	common.SendMessage(conn, msg)
}

func (b *Broker) sendReplyError(conn net.Conn, req *cellaserv.Request, errType cellaserv.Reply_Error_Type) {
	err := &cellaserv.Reply_Error{Type: errType}

	reply := &cellaserv.Reply{Error: err, Id: req.Id}
	replyBytes, _ := proto.Marshal(reply)

	msgType := cellaserv.Message_Reply
	msg := &cellaserv.Message{
		Type:    msgType,
		Content: replyBytes,
	}
	common.SendMessage(conn, msg)
}
