package web

import (
	"bytes"
	"context"
	"encoding/json"
	template "html/template"
	template_text "html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/pprof"
	"path"
	"strings"

	"bitbucket.org/evolutek/cellaserv3/broker"
	"bitbucket.org/evolutek/cellaserv3/broker/cellaserv/api"
	"bitbucket.org/evolutek/cellaserv3/client"
	"bitbucket.org/evolutek/cellaserv3/common"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/route"
	"github.com/rs/cors"
)

type Options struct {
	ListenAddr      string
	AssetsPath      string
	ExternalURLPath string
	BrokerAddr      string
}

// Handler represents the web component of cellaserv and holds references to
// cellaserv components used in the web interface.
type Handler struct {
	options *Options
	logger  common.Logger
	router  *route.Router
	broker  *broker.Broker
	client  *client.Client
}

func (h *Handler) makeRequestFromHTTP(r *http.Request) ([]byte, error) {
	// Extract request parameters
	service := route.Param(r.Context(), "service")
	service, identification := common.ParseServicePath(service)
	method := route.Param(r.Context(), "method")
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	// Make request
	serviceStub := client.NewServiceStub(h.client, service, identification)
	var resp []byte
	if len(body) == 0 {
		resp, err = serviceStub.RequestNoData(method)
	} else {
		resp, err = serviceStub.RequestRaw(method, body)
	}

	return resp, err
}

func (h *Handler) request(w http.ResponseWriter, r *http.Request) {
	requestData := struct {
	}{}
	h.executeTemplate(w, "request.html", requestData)
}

func (h *Handler) apiRequest(w http.ResponseWriter, r *http.Request) {
	resp, err := h.makeRequestFromHTTP(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write response
	_, err = w.Write(resp)
	if err != nil {
		h.logger.Errorf("Could not write response: %s", err)
	}
}

func (h *Handler) apiPublish(w http.ResponseWriter, r *http.Request) {
	// Extract request parameters
	event := route.Param(r.Context(), "event")
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.client.Publish(event, body)
}

var upgrader = websocket.Upgrader{
	// Allow all origins, this is ok because cellaserv is not exported
	// publicly
	CheckOrigin: func(r *http.Request) bool { return true },
}

// apiSubscribe handles websocket subscribes
func (h *Handler) apiSubscribe(w http.ResponseWriter, r *http.Request) {
	// Extract request parameters
	event := route.Param(r.Context(), "event")

	// Upgrade connection to websocket
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Could not upgrade:", err)
		return
	}
	defer c.Close()

	// TODO(halfr): handle websocket control messages and cancel subscribe
	// when the socket is closed
	err = h.client.SubscribeUntil(event,
		func(eventName string, eventBytes []byte) bool {
			msg := struct {
				Name string `json:"name"`
				Data string `json:"data"`
			}{Name: eventName, Data: string(eventBytes)}
			msgTxt, err := json.Marshal(msg)
			if err != nil {
				h.logger.Error("json:", err)
				return false
			}
			err = c.WriteMessage(websocket.TextMessage, msgTxt)
			if err != nil {
				h.logger.Error("Write error:", err)
				return true
			}
			return false
		})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	<-h.client.Quit()
}

// overview returns a page showing the list of connections, events and services
func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Serving overview")

	overview := struct {
		Clients  []api.ClientJSON
		Services []api.ServiceJSON
		Events   []api.EventInfoJSON
	}{
		Clients:  h.broker.GetClientsJSON(),
		Services: h.broker.GetServicesJSON(),
		Events:   h.broker.GetEventsJSON(),
	}

	h.executeTemplate(w, "overview.html", overview)
}

func (h *Handler) logs(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Serving logs")
	pattern := route.Param(r.Context(), "pattern")
	logs, err := h.broker.GetLogsByPattern(pattern)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.executeTemplate(w, "logs.html", logs)
}

func tmplFuncs(options *Options) template_text.FuncMap {
	return template_text.FuncMap{
		"pathPrefix": func() string { return options.ExternalURLPath },
	}
}

func (h *Handler) executeTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl := template.New("").Funcs(tmplFuncs(h.options))

	templatesPath := path.Join(h.options.AssetsPath, "templates")
	template.Must(tmpl.ParseFiles(
		path.Join(templatesPath, "_base.html"),
		path.Join(templatesPath, name)))

	var buffer bytes.Buffer
	err := tmpl.ExecuteTemplate(&buffer, "_base.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buffer.Bytes())
}

func serveDebug(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	subpath := route.Param(ctx, "subpath")

	if subpath == "/pprof" {
		http.Redirect(w, req, req.URL.Path+"/", http.StatusMovedPermanently)
		return
	}

	if !strings.HasPrefix(subpath, "/pprof/") {
		http.NotFound(w, req)
		return
	}
	subpath = strings.TrimPrefix(subpath, "/pprof/")

	switch subpath {
	case "cmdline":
		pprof.Cmdline(w, req)
	case "profile":
		pprof.Profile(w, req)
	case "symbol":
		pprof.Symbol(w, req)
	case "trace":
		pprof.Trace(w, req)
	default:
		req.URL.Path = "/debug/pprof/" + subpath
		pprof.Index(w, req)
	}
}

// Starts the web component
func (h *Handler) Run(ctx context.Context) error {
	// Wait for broker to be ready
	select {
	case <-h.broker.StartedWithCellaserv():
		break
	case <-ctx.Done():
		return nil
	}

	// Create cellaserv client that connects locally
	h.client = client.NewClient(client.ClientOpts{
		CellaservAddr: h.options.BrokerAddr,
		Name:          "web",
	})

	h.logger.Infof("Listening on http://%s", h.options.ListenAddr)
	handler := cors.Default().Handler(h.router)
	httpSrv := &http.Server{
		Addr:    h.options.ListenAddr,
		Handler: handler,
	}

	errChan := make(chan error)
	go func() {
		errChan <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-errChan:
		return err
	case <-h.client.Quit():
		return nil
	case <-ctx.Done():
		return httpSrv.Close()
	}
}

// Returns a new web endpoint Handler
func New(o *Options, logger common.Logger, broker *broker.Broker) *Handler {
	router := route.New()

	h := &Handler{
		options: o,
		logger:  logger,
		router:  router,
		broker:  broker,
	}

	// Setup HTTP endpoints
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/overview", http.StatusFound)
	})
	router.Get("/overview", h.overview)
	router.Get("/logs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/logs/*", http.StatusFound)
	})
	router.Get("/logs/:pattern", h.logs)
	router.Get("/request/:service/:method", h.request)
	router.Post("/request/:service/:method", h.request)
	router.Get("/static/*filepath", route.FileServe(path.Join(o.AssetsPath, "static")))

	router.Get("/metrics", promhttp.HandlerFor(prometheus.Gatherers{prometheus.DefaultGatherer, broker.Monitoring.Registry}, promhttp.HandlerOpts{}).ServeHTTP)

	// cellaserv HTTP API
	router.Get("/api/v1/request/:service/:method", h.apiRequest)
	router.Post("/api/v1/request/:service/:method", h.apiRequest)
	router.Post("/api/v1/publish/:event", h.apiPublish)
	router.Get("/api/v1/subscribe/:event", h.apiSubscribe)
	// TODO(halfr): spy

	router.Get("/debug/*subpath", serveDebug)
	router.Post("/debug/*subpath", serveDebug)

	return h
}
