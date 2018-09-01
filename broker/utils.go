package broker

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net"
	"strings"

	"bitbucket.org/evolutek/cellaserv2-protobuf"
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
func connDescribe(conn net.Conn) string {
	if name, ok := connNameMap[conn]; ok {
		return name
	}

	services, ok := servicesConn[conn]
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

func sendReply(conn net.Conn, req *cellaserv.Request, data []byte) {
	rep := &cellaserv.Reply{Id: req.Id, Data: data}
	repBytes, err := proto.Marshal(rep)
	if err != nil {
		log.Error("[Message] Could not marshal outgoing reply")
	}

	msgType := cellaserv.Message_Reply
	msg := &cellaserv.Message{Type: &msgType, Content: repBytes}

	sendMessage(conn, msg)
}

func sendReplyError(conn net.Conn, req *cellaserv.Request, errType cellaserv.Reply_Error_Type) {
	err := &cellaserv.Reply_Error{Type: &errType}

	reply := &cellaserv.Reply{Error: err, Id: req.Id}
	replyBytes, _ := proto.Marshal(reply)

	msgType := cellaserv.Message_Reply
	msg := &cellaserv.Message{
		Type:    &msgType,
		Content: replyBytes,
	}
	sendMessage(conn, msg)
}

func sendMessage(conn net.Conn, msg *cellaserv.Message) {
	log.Debug("[Net] Sending message to %s", conn.RemoteAddr())

	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		log.Error("[Message] Could not marshal outgoing message")
	}

	sendRawMessage(conn, msgBytes)
}

func sendRawMessage(conn net.Conn, msg []byte) {
	// Create temporary buffer
	var buf bytes.Buffer
	// Write the size of the message...
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(msg))); err != nil {
		log.Error("Could not write message to buffer:", err)
	}
	// ...concatenate with message content
	buf.Write(msg)
	// Send the whole message at once (avoid race condition)
	// Any IO error will be detected by the main loop trying to read from the conn
	if _, err := conn.Write(buf.Bytes()); err != nil {
		log.Error("Could not write message to connection:", err)
	}
}
