package broker

import (
	"encoding/json"
	"net"
	"strings"

	"bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/evolutek/cellaserv3/common"
	"github.com/golang/protobuf/proto"
)

// Log utils

type connJSON struct {
	Addr string
}

func connToJSON(conn net.Conn) []byte {
	ret, _ := json.Marshal(connJSON{conn.RemoteAddr().String()})
	return ret
}

// connDesribe returns all the information cellaserv have on the connection
func (b *Broker) connDescribe(conn net.Conn) string {
	if name, ok := b.connNameMap[conn]; ok {
		return name
	}

	services, ok := b.servicesConn[conn]
	if !ok {
		// This connection is not associated with a service
		return conn.RemoteAddr().String()
	}

	var servcs []string
	for _, srvc := range services {
		servcs = append(servcs, srvc.Name)
	}
	return strings.Join(servcs, ", ")
}

// Send utils

func (b *Broker) sendReply(conn net.Conn, req *cellaserv.Request, data []byte) {
	rep := &cellaserv.Reply{Id: req.Id, Data: data}
	repBytes, err := proto.Marshal(rep)
	if err != nil {
		b.logger.Error("[Message] Could not marshal outgoing reply")
	}

	msgType := cellaserv.Message_Reply
	msg := &cellaserv.Message{Type: &msgType, Content: repBytes}

	common.SendMessage(conn, msg)
}

func (b *Broker) sendReplyError(conn net.Conn, req *cellaserv.Request, errType cellaserv.Reply_Error_Type) {
	err := &cellaserv.Reply_Error{Type: &errType}

	reply := &cellaserv.Reply{Error: err, Id: req.Id}
	replyBytes, _ := proto.Marshal(reply)

	msgType := cellaserv.Message_Reply
	msg := &cellaserv.Message{
		Type:    &msgType,
		Content: replyBytes,
	}
	common.SendMessage(conn, msg)
}
