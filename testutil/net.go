package testutil

import (
	"net"
	"testing"

	cellaserv "github.com/evolutek/cellaserv3-protobuf"
	"github.com/evolutek/cellaserv3/common"
	"github.com/golang/protobuf/proto"
)

func Dial(t *testing.T) net.Conn {
	conn, err := net.Dial("tcp", ":4200")
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func RecvMessage(t *testing.T, conn net.Conn) *cellaserv.Message {
	closed, _, msg, err := common.RecvMessage(conn)
	if closed || err != nil {
		t.Error("Could not receive message:", err)
		return nil
	}
	return msg
}

func RecvReply(t *testing.T, conn net.Conn) []byte {
	msg := RecvMessage(t, conn)
	MsgTypeIs(t, msg, cellaserv.Message_Reply)
	repData := msg.Content
	var rep cellaserv.Reply
	err := proto.Unmarshal(repData, &rep)
	Ok(t, err)
	return rep.Data
}
