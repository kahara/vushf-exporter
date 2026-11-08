package main

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

const (
	Namespace = "pskreporter"
	Subsystem = "spots"
)

var (
	sent_metric     *prometheus.CounterVec
	received_metric *prometheus.CounterVec
	local_metric    *prometheus.CounterVec
)

func SetupMetrics() {
	sent_metric = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "sent_total",
	}, []string{"country", "band", "mode"})

	received_metric = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "received_total",
	}, []string{"country", "band", "mode"})

	local_metric = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "local_total",
	}, []string{"country", "band", "mode"})
}

func Metrics(addrPort string) {
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		promhttp.Handler().ServeHTTP(w, r)
	})
	metricsServer := &http.Server{
		Addr:              addrPort,
		Handler:           metricsMux,
		ReadTimeout:       2 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      5 * time.Second,
	}
	log.Fatal().Err(metricsServer.ListenAndServe()).Send()
}
