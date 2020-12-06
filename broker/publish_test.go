package broker

import (
	"testing"
	"time"

	cellaserv "github.com/evolutek/cellaserv3-protobuf"
	"github.com/evolutek/cellaserv3/testutil"
	"github.com/golang/protobuf/proto"
)

func TestPublishNoSubscriber(t *testing.T) {
	brokerTest(t, func(b *Broker) {
		conn := testutil.Dial(t)
		defer conn.Close()

		const topic = "test"
		conn.Write(testutil.MakeMessagePublish(t, topic))

		time.Sleep(50 * time.Millisecond)
	})
}

func TestPublish(t *testing.T) {
	brokerTest(t, func(b *Broker) {
		conn := testutil.Dial(t)
		defer conn.Close()

		const topic = "test"
		conn.Write(testutil.MakeMessageSubscribe(t, topic))
		time.Sleep(50 * time.Millisecond)
		conn.Write(testutil.MakeMessagePublish(t, topic))
		time.Sleep(50 * time.Millisecond)

		// Read publish
		msg := testutil.RecvMessage(t, conn)
		// Check publish
		testutil.MsgTypeIs(t, msg, cellaserv.Message_Publish)
		msgPublish := &cellaserv.Publish{}
		msgContent := msg.GetContent()
		if err := proto.Unmarshal(msgContent, msgPublish); err != nil {
			t.Error("Could not unmarshal message content:", err)
			return
		}
		testutil.Equals(t, msgPublish.GetEvent(), topic)
	})
}

func TestPublishPattern(t *testing.T) {
	brokerTest(t, func(b *Broker) {
		conn := testutil.Dial(t)
		defer conn.Close()

		const pattern = "test*"
		conn.Write(testutil.MakeMessageSubscribe(t, pattern))
		time.Sleep(50 * time.Millisecond)
		const topic = "test.foobarlol"
		conn.Write(testutil.MakeMessagePublish(t, topic))
		time.Sleep(50 * time.Millisecond)

		// Read publish
		msg := testutil.RecvMessage(t, conn)
		// Check publish
		testutil.MsgTypeIs(t, msg, cellaserv.Message_Publish)
		msgPublish := &cellaserv.Publish{}
		msgContent := msg.GetContent()
		if err := proto.Unmarshal(msgContent, msgPublish); err != nil {
			t.Error("Could not unmarshal message content:", err)
			return
		}
		testutil.Equals(t, msgPublish.GetEvent(), topic)
	})
}
