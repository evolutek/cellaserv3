package web

import (
	"context"
	"encoding/json"
	template "html/template"
	template_text "html/template"
	"net/http"
	"path"

	"github.com/evolutek/cellaserv3/broker"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/route"
	logging "gopkg.in/op/go-logging.v1"
)

type Options struct {
	TemplatesPath   string
	StaticsPath     string
	ExternalURLPath string
}

// Handler represents the web component of cellaserv and holds references to
// cellaserv components used in the web interface.
type Handler struct {
	options *Options
	logger  *logging.Logger
	router  *route.Router
	broker  *broker.Broker

	// TODO(halfr): add ready bit
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	overview := struct {
		Connections []broker.ConnNameJSON
		Services    []*broker.ServiceJSON
		Events      broker.EventInfoJSON
	}{
		Connections: h.broker.GetConnectionList(),
		Services:    h.broker.GetServiceList(),
		Events:      h.broker.GetSubscribersInfo(),
	}

	h.executeTemplate(w, "overview.html", overview)
}

func tmplFuncs(options *Options) template_text.FuncMap {
	return template_text.FuncMap{
		"pathPrefix": func() string { return options.ExternalURLPath },
	}
}

func (h *Handler) executeTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl := template.New("").Funcs(tmplFuncs(h.options))

	template.Must(tmpl.ParseFiles(
		path.Join(h.options.TemplatesPath, "_base.html"),
		path.Join(h.options.TemplatesPath, name)))

	err := tmpl.ExecuteTemplate(w, "_base.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Starts the web component
func (h *Handler) Run(ctx context.Context) error {
	// TODO(halfr) put that in h.Option
	httpListenAddr := ":4280"

	h.logger.Info("[Web] Listening on %s", httpListenAddr)

	httpSrv := &http.Server{
		Addr:    httpListenAddr,
		Handler: h.router,
	}
	return httpSrv.ListenAndServe()
}

// Returns a new web endpoint Handler
func New(o *Options, logger *logging.Logger, broker *broker.Broker) *Handler {
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

	router.Get("/static/*filepath", route.FileServe(o.StaticsPath))

	// API endpoints
	// TODO(halfr): remove if not used
	router.Get("/api/services/list", func(w http.ResponseWriter, r *http.Request) {
		// TODO(halfr): do not dump the internal state and return a
		// list of services
		dumpJSON, _ := json.Marshal(h.broker.Services)
		w.Write(dumpJSON)
	})

	// Prometheus metrics of the broker
	router.Get("/metrics", promhttp.Handler().ServeHTTP)

	return h
}
