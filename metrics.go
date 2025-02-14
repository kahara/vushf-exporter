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
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(addrPort, nil); err != nil {
		log.Fatal().Err(err).Str("addrport", addrPort).Msg("Could not expose Prometheus metrics")
	}
}
