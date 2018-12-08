package broker

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"syscall"
	"testing"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"bitbucket.org/evolutek/cellaserv3/testutil"
	"github.com/golang/protobuf/proto"
)

func TestPublishLog(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "testcellaserv")
	testutil.Ok(t, err)
	defer syscall.Unlink(tmpDir)

	brokerTestWithOptions(t, Options{
		VarRoot:               tmpDir,
		PublishLoggingEnabled: true,
	}, func(b *Broker) {
		connA, connB := net.Pipe()

		go func() {
			pub := &cellaserv.Publish{
				Event: "log.test",
				Data:  []byte("coucou"),
			}
			pubMsg, err := proto.Marshal(pub)
			testutil.Ok(t, err)
			b.doPublish(pubMsg, pub)

			req := &cellaserv.Request{
				ServiceName: "cellaserv",
				Method:      "get_logs",
				Data:        []byte("test"),
			}
			b.handleGetLogs(connA, req)
		}()

		repDataBytes := testutil.RecvReply(t, connB)
		repData := make(map[string]string)
		err := json.Unmarshal(repDataBytes, &repData)
		testutil.Ok(t, err)
		testutil.Equals(t, repData, map[string]string{"test": "coucou\n"})
	})
}
