package web

import (
	"context"
	template "html/template"
	template_text "html/template"
	"net/http"
	"path"

	"github.com/evolutek/cellaserv3/broker"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/route"
	logging "gopkg.in/op/go-logging.v1"
)

type Options struct {
	ListenAddr      string
	AssetsPath      string
	ExternalURLPath string
}

// Handler represents the web component of cellaserv and holds references to
// cellaserv components used in the web interface.
type Handler struct {
	options *Options
	logger  *logging.Logger
	router  *route.Router
	broker  *broker.Broker
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	overview := struct {
		Connections []broker.ConnectionJSON
		Services    []broker.ServiceJSON
		Events      broker.EventsJSON
	}{
		Connections: h.broker.GetConnectionsJSON(),
		Services:    h.broker.GetServicesJSON(),
		Events:      h.broker.GetEventsJSON(),
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

	templatesPath := path.Join(h.options.AssetsPath, "templates")
	template.Must(tmpl.ParseFiles(
		path.Join(templatesPath, "_base.html"),
		path.Join(templatesPath, name)))

	err := tmpl.ExecuteTemplate(w, "_base.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Starts the web component
func (h *Handler) Run(ctx context.Context) error {
	h.logger.Info("[Web] Listening on %s", h.options.ListenAddr)

	httpSrv := &http.Server{
		Addr:    h.options.ListenAddr,
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
	router.Get("/static/*filepath", route.FileServe(path.Join(o.AssetsPath, "static")))
	router.Get("/metrics", promhttp.HandlerFor(prometheus.Gatherers{prometheus.DefaultGatherer, broker.Monitoring.Registry}, promhttp.HandlerOpts{}).ServeHTTP)

	return h
}
