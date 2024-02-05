package main

import (
	"github.com/rs/zerolog/log"
)

func main() {
	var config = NewConfig()

	log.Debug().Any("config", config).Msg("")

	SetupMetrics()
	go Metrics(config.MetricAddr)

	Subscribe(*config)
}
