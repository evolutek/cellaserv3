package cellaserv

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"syscall"
	"testing"
	"time"

	"bitbucket.org/evolutek/cellaserv3/broker"
	"bitbucket.org/evolutek/cellaserv3/broker/cellaserv/api"
	"bitbucket.org/evolutek/cellaserv3/client"
	"bitbucket.org/evolutek/cellaserv3/common"
	"bitbucket.org/evolutek/cellaserv3/testutil"
)

func WithTestBrokerOptions(t *testing.T, options broker.Options, testFn func(client.ClientOpts)) {
	ctxBroker, cancelBroker := context.WithCancel(context.Background())
	ctxCellaserv, cancelCellaserv := context.WithCancel(context.Background())
	broker := broker.New(options, common.NewLogger("broker"))
	cs := New(&Options{BrokerAddr: options.ListenAddress}, broker, common.NewLogger("cellaserv"))

	go func() {
		err := broker.Run(ctxBroker)
		if err != nil {
			t.Fatalf("Could not start broker: %s", err)
		}
	}()

	go func() {
		err := cs.Run(ctxCellaserv)
		if err != nil {
			t.Fatalf("Could not start cellaserv: %s", err)
		}
	}()

	<-broker.Started()
	<-cs.registeredCh

	// Run the test
	testFn(client.ClientOpts{CellaservAddr: options.ListenAddress})
	time.Sleep(50 * time.Millisecond)

	// Teardown broker
	cancelCellaserv()
	cancelBroker()
	time.Sleep(50 * time.Millisecond)
}

func TestPublishLog(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "testcellaserv")
	testutil.Ok(t, err)
	defer syscall.Unlink(tmpDir)

	WithTestBrokerOptions(t, broker.Options{
		ListenAddress:         ":4203",
		LogsDir:               tmpDir,
		PublishLoggingEnabled: true,
	}, func(clientOpts client.ClientOpts) {
		c := client.NewClient(clientOpts)
		publishData := "coucou"
		c.Publish("log.test_publish", publishData)
		cs := client.NewServiceStub(c, "cellaserv", "")
		respDataBytes, err := cs.Request("get_logs", api.GetLogsRequest{"test_pub*"})
		var respData map[string]string
		err = json.Unmarshal(respDataBytes, &respData)
		testutil.Ok(t, err)
		publishDataJson, _ := json.Marshal(publishData)
		testutil.Equals(t, respData, map[string]string{"test_publish": string(publishDataJson) + "\n"})
	})
}
