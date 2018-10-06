package common

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
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

func RecvMessage(conn net.Conn) (closed bool, msgBytes []byte, msg *cellaserv.Message, err error) {
	// Read message length as uint32
	var msgLen uint32
	err = binary.Read(conn, binary.BigEndian, &msgLen)
	if err != nil {
		if err == io.EOF {
			return true, nil, nil, nil
		}
		err = fmt.Errorf("Could not read message length: %s", err)
		return
	}

	const maxMessageSize = 8 * 1024 * 1024
	if msgLen > maxMessageSize {
		err = fmt.Errorf("Message size too big: %d, max size: %d", msgLen, maxMessageSize)
		return
	}

	// Extract message from connection
	msgBytes = make([]byte, msgLen)
	_, err = conn.Read(msgBytes)
	if err != nil {
		err = fmt.Errorf("Could not read message: %s", err)
		return
	}

	// Parse message header
	msg = &cellaserv.Message{}
	err = proto.Unmarshal(msgBytes, msg)
	if err != nil {
		err = fmt.Errorf("Could not unmarshal message: %s", err)
		return
	}

	return
}
