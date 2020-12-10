package cellaserv

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"syscall"
	"testing"
	"time"

	"github.com/evolutek/cellaserv3/broker"
	"github.com/evolutek/cellaserv3/broker/cellaserv/api"
	"github.com/evolutek/cellaserv3/client"
	"github.com/evolutek/cellaserv3/common"
	"github.com/evolutek/cellaserv3/testutil"
)

func WithTestBrokerOptions(t *testing.T, options broker.Options, testFn func(client.ClientOpts, *broker.Broker)) {
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
	<-cs.Registered()

	// Run the test
	testFn(client.ClientOpts{CellaservAddr: options.ListenAddress}, broker)
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
	}, func(clientOpts client.ClientOpts, broker *broker.Broker) {
		c := client.NewClient(clientOpts)
		publishData := "coucou"
		c.Publish("log.test_publish", publishData)
		cs := client.NewServiceStub(c, "cellaserv", "")
		respDataBytes, err := cs.Request("get_logs", api.GetLogsRequest{
			Pattern: "test_pub*",
		})
		testutil.Ok(t, err)
		var respData map[string]string
		err = json.Unmarshal(respDataBytes, &respData)
		testutil.Ok(t, err)
		publishDataJson, _ := json.Marshal(publishData)
		testutil.Equals(t, respData, map[string]string{"test_publish": string(publishDataJson) + "\n"})
	})
}

func listServices(t *testing.T, cs *client.ServiceStub) []api.ServiceJSON {
	respDataBytes, err := cs.Request("list_services", nil)
	testutil.Ok(t, err)
	var resp []api.ServiceJSON
	err = json.Unmarshal(respDataBytes, &resp)
	testutil.Ok(t, err)
	return resp
}

func TestRegisterRequest(t *testing.T) {
	WithTestBrokerOptions(t, broker.Options{
		ListenAddress: ":4203",
	}, func(clientOpts client.ClientOpts, broker *broker.Broker) {
		c := client.NewClient(clientOpts)
		cs := client.NewServiceStub(c, "cellaserv", "")

		servicesPre := listServices(t, cs)

		// Register
		respDataBytes, err := cs.Request("register_service", api.RegisterServiceRequest{
			Name:           "test_service",
			Identification: "test_identification",
		})
		testutil.Ok(t, err)
		testutil.Equals(t, []byte("null"), respDataBytes)

		servicesPost := listServices(t, cs)

		testutil.Equals(t, len(servicesPre)+1, len(servicesPost))

	})
}
