package web

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/evolutek/cellaserv3/broker"
	"github.com/evolutek/cellaserv3/broker/cellaserv"
	"github.com/evolutek/cellaserv3/common"
	"github.com/evolutek/cellaserv3/testutil"
)

func TestWeb(t *testing.T) {
	brokerOptions := broker.Options{ListenAddress: ":4204"}
	broker := broker.New(brokerOptions, common.NewLogger("broker"))

	csOpts := &cellaserv.Options{BrokerAddr: ":4204"}
	cs := cellaserv.New(csOpts, broker, common.NewLogger("cellaserv"))

	go func() {
		if err := broker.Run(context.Background()); err != nil {
			panic(fmt.Sprintf("Could not start broker: %s", err))
		}
	}()

	go func() {
		if err := cs.Run(context.Background()); err != nil {
			panic(fmt.Errorf("[Cellaserv] Could not start: %s", err))
		}
	}()

	opts := &Options{
		BrokerAddr: ":4204",
		ListenAddr: ":4284",
		AssetsPath: "ui",
	}
	webHandler := New(opts, common.NewLogger("web"), broker)

	go func() {
		err := webHandler.Run(context.Background())
		if err != nil {
			panic(fmt.Sprintf("Could not start web handler: %s", err))
		}
	}()

	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get("http://localhost:4284")
	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:4284/overview")
	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:4284/metrics")
	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)
}
