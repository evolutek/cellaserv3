package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	template "html/template"
	template_text "html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/pprof"
	"path"
	"strings"
	"time"

	"github.com/evolutek/cellaserv3/broker"
	"github.com/evolutek/cellaserv3/broker/cellaserv/api"
	"github.com/evolutek/cellaserv3/client"
	"github.com/evolutek/cellaserv3/common"

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

type requestTemplateData struct {
	Name           string
	Identification string
	Method         string
	Arguments      string
	Response       string
}

func (h *Handler) handleRequest(w http.ResponseWriter, r *http.Request) {
	requestData := requestTemplateData{
		Name:           r.FormValue("name"),
		Identification: r.FormValue("identification"),
		Method:         r.FormValue("method"),
		Arguments:      r.FormValue("arguments"),
	}

	h.executeTemplate(w, "request.html", requestData)
}

func (h *Handler) handleRequestPost(w http.ResponseWriter, r *http.Request) {
	requestData := requestTemplateData{
		Name:           r.PostFormValue("name"),
		Identification: r.PostFormValue("identification"),
		Method:         r.PostFormValue("method"),
		Arguments:      r.PostFormValue("arguments"),
	}

	serviceStub := client.NewServiceStub(h.client, requestData.Name, requestData.Identification)

	resp, err := serviceStub.RequestRaw(requestData.Method, []byte(requestData.Arguments))

	if err != nil {
		requestData.Response = err.Error()
	} else {
		requestData.Response = string(resp)
	}

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

// WebSocket constants
const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

func (h *Handler) ping(ws *websocket.Conn, done chan struct{}) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait)); err != nil {
				h.logger.Errorf("ping: %s", err)
				done <- struct{}{}
			}
		case <-done:
			return
		}
	}
}

// apiSubscribe handles websocket subscribes
func (h *Handler) apiSubscribe(w http.ResponseWriter, r *http.Request) {
	// Extract request parameters
	event := route.Param(r.Context(), "event")

	// Upgrade connection to websocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Could not upgrade:", err)
		return
	}
	defer ws.Close()

	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	done := make(chan struct{})

	go h.ping(ws, done)

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
			err = ws.WriteMessage(websocket.TextMessage, msgTxt)
			if err != nil {
				h.logger.Error("Write error:", err)
				done <- struct{}{}
				return true
			}
			return false
		})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	<-done
}

// overview returns a page showing the list of connections, events and services
func (h *Handler) handleOverview(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleLogs(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Serving logs")
	pattern := route.Param(r.Context(), "pattern")
	logs, err := h.broker.GetLogsByPattern(pattern)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		LogName string
		Logs    map[string]string
	}{
		LogName: pattern,
		Logs:    logs,
	}

	h.executeTemplate(w, "logs.html", data)
}

func tmplFuncs(options *Options, templateName string) template_text.FuncMap {
	return template_text.FuncMap{
		"pathPrefix":   func() string { return options.ExternalURLPath },
		"templateName": func() string { return templateName },
	}
}

func (h *Handler) executeTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl := template.New("").Funcs(tmplFuncs(h.options, name))

	templatesPath := path.Join(h.options.AssetsPath, "templates")
	_, err := tmpl.ParseFiles(
		path.Join(templatesPath, "_base.html"),
		path.Join(templatesPath, name))
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not load templates: %v", err), http.StatusInternalServerError)
		return
	}

	var buffer bytes.Buffer
	err = tmpl.ExecuteTemplate(&buffer, "_base.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buffer.Bytes())
}

func handleDebug(w http.ResponseWriter, req *http.Request) {
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
	router.Get("/overview", h.handleOverview)
	router.Get("/logs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/logs/*", http.StatusFound)
	})
	router.Get("/logs/:pattern", h.handleLogs)
	router.Get("/request", h.handleRequest)
	router.Post("/request", h.handleRequestPost)

	// Static files
	router.Get("/static/*filepath", route.FileServe(path.Join(o.AssetsPath, "static")))

	// Prometheus HTTP endpoint
	router.Get("/metrics", promhttp.HandlerFor(prometheus.Gatherers{prometheus.DefaultGatherer, broker.Monitoring.Registry}, promhttp.HandlerOpts{}).ServeHTTP)

	// cellaserv HTTP API
	router.Get("/api/v1/request/:service/:method", h.apiRequest)
	router.Post("/api/v1/request/:service/:method", h.apiRequest)
	router.Post("/api/v1/publish/:event", h.apiPublish)
	router.Get("/api/v1/subscribe/:event", h.apiSubscribe)
	// TODO(halfr): spy

	// Go debug
	router.Get("/debug/*subpath", handleDebug)
	router.Post("/debug/*subpath", handleDebug)

	return h
}
