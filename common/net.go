package common

import (
	"bytes"
	"encoding/binary"
	"net"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/golang/protobuf/proto"
)

func SendMessage(conn net.Conn, msg *cellaserv.Message) {
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		log.Error("[Message] Could not marshal outgoing message")
	}

	SendRawMessage(conn, msgBytes)
}

func SendRawMessage(conn net.Conn, msg []byte) {
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
