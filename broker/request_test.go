package broker

import (
	"bytes"
	"testing"
	"time"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/evolutek/cellaserv3/common"
	"github.com/evolutek/cellaserv3/testutil"
	"github.com/golang/protobuf/proto"
)

func TestRequestToService(t *testing.T) {
	go func() {
		defer handleShutdown()

		connService := testutil.Dial(t)
		defer connService.Close()

		connClient := testutil.Dial(t)
		defer connClient.Close()

		// Register first service
		const serviceName = "testName"
		const serviceIdent = "testIdent"
		registerMsg := testutil.MakeMessageRegister(t, serviceName, serviceIdent)
		connService.Write(registerMsg)

		time.Sleep(50 * time.Millisecond)

		// Sent request to service
		var payload []byte
		requestMsg := testutil.MakeMessageRequest(t, serviceName, serviceIdent, "method", payload)
		connClient.Write(requestMsg)

		// The service receives message
		closed, _, msg, err := common.RecvMessage(connService)
		if closed || err != nil {
			t.Error(err)
			return
		}
		if msg.GetType() != cellaserv.Message_Request {
			t.Error("Unknown message type: ", msg.GetType())
			return
		}

		// The service receives a request
		msgRequest := &cellaserv.Request{}
		msgContent := msg.GetContent()
		if err = proto.Unmarshal(msgContent, msgRequest); err != nil {
			t.Error("Could not unmarshal message content:", err)
			return
		}

		// The service sends a reply
		payloadReply := []byte{42, 42}
		replyMsgBytes := testutil.MakeMessageReply(t, msgRequest.GetId(), payloadReply)
		connService.Write(replyMsgBytes)

		// The client receives reply
		closed, _, msg, err = common.RecvMessage(connClient)
		if closed || err != nil {
			t.Error(err)
			return
		}
		if msg.GetType() != cellaserv.Message_Reply {
			t.Error("Wrong message type:", msg.GetType())
			return
		}
		msgReply := &cellaserv.Reply{}
		msgContent = msg.GetContent()
		if err = proto.Unmarshal(msgContent, msgReply); err != nil {
			t.Error("Could not unmarshal message content:", err)
			return
		}
		if !bytes.Equal(msgReply.GetData(), payloadReply) {
			t.Error("Wrong message reply content:", msgReply.GetData())
			return
		}

		time.Sleep(50 * time.Millisecond)
	}()

	listenAndServeForTest(t)
}

func TestRequestNoService(t *testing.T) {
	go func() {
		defer handleShutdown()

		conn := testutil.Dial(t)
		defer conn.Close()

		var payload []byte
		msg := testutil.MakeMessageRequest(t, "foo", "bar", "lol", payload)
		conn.Write(msg)

		time.Sleep(50 * time.Millisecond)
	}()

	listenAndServeForTest(t)
}
