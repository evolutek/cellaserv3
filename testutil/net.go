package testutil

import (
	"net"
	"testing"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/evolutek/cellaserv3/common"
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
