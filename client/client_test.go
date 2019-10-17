package client

import (
	"bytes"
	"encoding/json"
	"net"
	"testing"
	"time"

	cellaserv "bitbucket.org/evolutek/cellaserv3-protobuf"
	"bitbucket.org/evolutek/cellaserv3/common"
	"github.com/golang/protobuf/proto"
)

func TestNewClient(t *testing.T) {
	_, client := net.Pipe()
	c := newClient(client, "test")
	c.Close()
}

func TestRegisterService(t *testing.T) {
	server, client := net.Pipe()

	// Dummy server setup
	defer server.Close()
	go func() {
		buf := make([]byte, 1)

		for {
			server.Read(buf)
		}
	}()

	// Connect to cellaserv
	conn := newClient(client, "") // no name
	// TODO(halfr): test with a name

	// Prepare service for registration
	date := conn.NewService("date", "")
	// Handle "time" request
	date.HandleRequestFunc("time", func(_ *cellaserv.Request) (interface{}, error) {
		return time.Now(), nil
	})
	// Handle "killall" event
	date.HandleEventFunc("killall", func(_ *cellaserv.Publish) {
		conn.Close()
	})

	// Register the service on cellaserv
	conn.RegisterService(date)

	conn.Close()
}

func TestServiceStubRequest(t *testing.T) {
	server, client := net.Pipe()

	expectedReplyData := []byte{42, 42}

	go func() {
		// Receive request
		_, _, msg, err := common.RecvMessage(server)
		if err != nil {
			t.Error(err)
			return
		}
		if msg.GetType() != cellaserv.Message_Request {
			t.Errorf("Invalid message type, should be Request, is: %s", msg.GetType())
			return
		}
		// Parse request
		msgContent := msg.GetContent()
		var req cellaserv.Request
		err = proto.Unmarshal(msgContent, &req)
		if err != nil {
			t.Error(err)
			return
		}

		// Make reply
		id := req.GetId()
		reply := &cellaserv.Reply{
			Id:   id,
			Data: expectedReplyData,
		}

		msgContent, _ = proto.Marshal(reply)
		msgType := cellaserv.Message_Reply
		replyMsg := &cellaserv.Message{Type: msgType, Content: msgContent}
		common.SendMessage(server, replyMsg)
	}()

	c := newClient(client, "test")
	// Create date service stub
	date := NewServiceStub(c, "date", "")
	// Request date.time()
	replyData, err := date.Request("time", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(replyData, expectedReplyData) {
		t.Fatal("Invalid reply")
	}
}

func TestPublish(t *testing.T) {
	server, client := net.Pipe()

	done := make(chan struct{})

	publishEvent := "foo"
	publishData := "qwerqwer"

	go func() {
		defer close(done)

		// Receive publish
		_, _, msg, err := common.RecvMessage(server)
		if err != nil {
			t.Error(err)
			return
		}
		if msg.GetType() != cellaserv.Message_Publish {
			t.Errorf("Invalid message type, should be Publish, is: %s", msg.GetType())
			return
		}

		// Parse publish
		msgContent := msg.GetContent()
		var pub cellaserv.Publish
		err = proto.Unmarshal(msgContent, &pub)
		if err != nil {
			t.Error(err)
			return
		}

		if pub.GetEvent() != publishEvent {
			t.Errorf("Invalid event name, expected %s, is %s", publishEvent, pub.GetEvent())
			return
		}

		var pubDataJSON string
		json.Unmarshal(pub.GetData(), &pubDataJSON)
		if pubDataJSON != publishData {
			t.Errorf("Invalid event data, expected %s, is %s", publishData, pub.GetData())
			return
		}
	}()

	c := newClient(client, "test")
	c.Publish(publishEvent, publishData)
	<-done
}
