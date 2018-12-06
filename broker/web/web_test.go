package web

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"bitbucket.org/evolutek/cellaserv3/broker"
	"github.com/prometheus/prometheus/util/testutil"
	logging "github.com/op/go-logging"
)

func TestWeb(t *testing.T) {
	brokerOptions := &broker.Options{ListenAddress: ":4200"}
	broker := broker.New(brokerOptions, logging.MustGetLogger("broker"))

	opts := &Options{
		ListenAddr: ":4280",
		AssetsPath: "ui",
	}
	webHandler := New(opts, logging.MustGetLogger("web"), broker)

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
}
