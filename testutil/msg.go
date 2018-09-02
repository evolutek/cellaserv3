package testutil

import (
	"bytes"
	"encoding/binary"
	"testing"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/golang/protobuf/proto"
)

func MessageForNetwork(t *testing.T, msg *cellaserv.Message) []byte {
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal("Could not marshal outgoing message")
	}

	var buf bytes.Buffer
	// Write the size of the message...
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(msgBytes))); err != nil {
		t.Fatal("Could not write message to buffer:", err)
	}
	// ...concatenate with message content
	buf.Write(msgBytes)

	return buf.Bytes()
}

func MakeMessageRegister(t *testing.T, serviceName string) []byte {
	msgType := cellaserv.Message_Register
	msgContent := &cellaserv.Register{Name: &serviceName}
	msgContentBytes, _ := proto.Marshal(msgContent)
	msg := &cellaserv.Message{Type: &msgType, Content: msgContentBytes}
	return MessageForNetwork(t, msg)
}

func MakeMessagePublish(t *testing.T, topic string) []byte {
	msgType := cellaserv.Message_Publish
	msgContent := &cellaserv.Publish{Event: &topic}
	msgContentBytes, _ := proto.Marshal(msgContent)
	msg := &cellaserv.Message{Type: &msgType, Content: msgContentBytes}
	return MessageForNetwork(t, msg)
}
