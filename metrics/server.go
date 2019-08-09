package metrics

import (
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusConfig defines a config for a metrics server
type PrometheusConfig struct {
	Path string `json:"path,omitempty"`
	Port string `json:"port,omitempty"`
}

// RunServer starts a metrics server
func RunServer(config PrometheusConfig, registry *prometheus.Registry, log logr.Logger) {
	mux := http.NewServeMux()
	mux.Handle(config.Path, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	srv := &http.Server{Addr: fmt.Sprintf("%s", config.Port), Handler: mux}

	defer func() {
		if cerr := srv.Close(); cerr != nil {
			log.Error(cerr, "Encountered an error when trying to close the metrics server")
		}
	}()

	log.Info("Starting prometheus metrics server at 0.0.0.0", "port", config.Port, "path", config.Path)
	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}
