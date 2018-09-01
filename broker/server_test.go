package broker

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/evolutek/cellaserv3/common"
	"github.com/golang/protobuf/proto"
)

func TestServerHandleRegisterMsg(t *testing.T) {
	common.LogSetup()

	go func() {
		conn, err := net.Dial("tcp", ":4200")
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		testServiceName := "test"

		msgType := cellaserv.Message_Register
		msgContent := &cellaserv.Register{Name: &testServiceName}
		msgContentBytes, _ := proto.Marshal(msgContent)
		msg := &cellaserv.Message{Type: &msgType, Content: msgContentBytes}
		msgBytes, err := proto.Marshal(msg)
		if err != nil {
			log.Error("[Message] Could not marshal outgoing message")
		}

		var buf bytes.Buffer
		// Write the size of the message...
		if err := binary.Write(&buf, binary.BigEndian, uint32(len(msgBytes))); err != nil {
			log.Error("Could not write message to buffer:", err)
		}
		// ...concatenate with message content
		buf.Write(msgBytes)

		// Send message!
		conn.Write(buf.Bytes())

		time.Sleep(50 * time.Millisecond)

		if len(services) != 1 {
			t.Fail()
		}

		// Leave server
		handleShutdown()

		time.Sleep(50 * time.Millisecond)
	}()

	Serve()
}
