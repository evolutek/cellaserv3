package web

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"bitbucket.org/evolutek/cellaserv3/broker"
	"bitbucket.org/evolutek/cellaserv3/common"
	"github.com/prometheus/prometheus/util/testutil"
)

func TestWeb(t *testing.T) {
	brokerOptions := broker.Options{ListenAddress: ":4200"}
	broker := broker.New(brokerOptions, common.NewLogger("broker"))

	opts := &Options{
		ListenAddr: ":4280",
		AssetsPath: "ui",
	}
	webHandler := New(opts, common.NewLogger("web"), broker)

	go func() {
		err := broker.Run(context.Background())
		if err != nil {
			panic(fmt.Sprintf("Could not start broker: %s", err))
		}
	}()

	go func() {
		err := webHandler.Run(context.Background())
		if err != nil {
			panic(fmt.Sprintf("Could not start web handler: %s", err))
		}
	}()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:4280")
	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:4280/overview")
	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:4280/metrics")
	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)
}
