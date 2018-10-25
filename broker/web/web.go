package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/evolutek/cellaserv3/broker"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/route"
	logging "gopkg.in/op/go-logging.v1"
)

type Handler struct {
	logger *logging.Logger
	router *route.Router

	broker *broker.Broker

	// TODO(halfr): add ready
}

func (h *Handler) Run(ctx context.Context) error {
	// TODO(halfr) put that in h.Option
	httpListenAddr := ":4280"

	log.Info("[HTTP] Listening on %s", httpListenAddr)
	httpSrv := &http.Server{
		Addr:    httpListenAddr,
		Handler: h.router,
	}
	return httpSrv.ListenAndServe()
}

func New(logger *logging.Logger, broker *broker.Broker) *Handler {
	router := route.New()

	h := &Handler{
		logger: logger,
		router: router,

		broker: broker,
	}

	// Setup HTTP endpoints
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Cellaserv!\n")
	})
	router.Get("/api/services", func(w http.ResponseWriter, r *http.Request) {
		dumpJSON, _ := json.Marshal(h.broker.Services)
		w.Write(dumpJSON)
	})
	router.Get("/metrics", promhttp.Handler().ServeHTTP)

	return h
}
