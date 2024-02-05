package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"net/http"
)

const (
	Namespace = "pskreporter"
	Subsystem = "spots"
)

var (
	sent_metric     *prometheus.CounterVec
	received_metric *prometheus.CounterVec
)

func SetupMetrics() {
	sent_metric = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "sent_total",
	}, []string{"country", "band"})

	received_metric = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "received_total",
	}, []string{"country", "band"})
}

func Metrics(metricsAddr string) {
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(metricsAddr, nil); err != nil {
		log.Fatal().Err(err).Str("addrport", metricsAddr).Msg("Could not expose Prometheus metrics")
	}
}
