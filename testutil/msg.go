package testutil

import (
	"bytes"
	"encoding/binary"
	"sync/atomic"
	"testing"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/golang/protobuf/proto"
)

var NextMessageRequestId uint64

func makeMessage(t *testing.T, msgType cellaserv.Message_MessageType, msgContent proto.Message) []byte {
	msgContentBytes, err := proto.Marshal(msgContent)
	if err != nil {
		t.Fatal("Protobuf marshalling error:", err)
	}
	msg := &cellaserv.Message{Type: msgType, Content: msgContentBytes}
	return MessageForNetwork(t, msg)
}

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

func MakeMessageRegister(t *testing.T, serviceName string, serviceIdent string) []byte {
	msgType := cellaserv.Message_Register
	msgContent := &cellaserv.Register{
		Name:           serviceName,
		Identification: serviceIdent,
	}
	return makeMessage(t, msgType, msgContent)
}

func MakeMessagePublish(t *testing.T, topic string) []byte {
	msgType := cellaserv.Message_Publish
	msgContent := &cellaserv.Publish{Event: topic}
	return makeMessage(t, msgType, msgContent)
}

func MakeMessageRequest(t *testing.T, service string, ident string, method string, payload []byte) []byte {
	msgType := cellaserv.Message_Request
	msgId := atomic.AddUint64(&NextMessageRequestId, 1)
	msgContent := &cellaserv.Request{
		ServiceIdentification: ident,
		ServiceName:           service,
		Method:                method,
		Data:                  payload,
		Id:                    msgId,
	}
	return makeMessage(t, msgType, msgContent)
}

func MakeMessageReply(t *testing.T, msgId uint64, payload []byte) []byte {
	msgType := cellaserv.Message_Reply
	msgContent := &cellaserv.Reply{
		Id:   msgId,
		Data: payload,
	}
	return makeMessage(t, msgType, msgContent)
}
