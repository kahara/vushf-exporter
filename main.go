package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"time"
)

func main() {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	var config = NewConfig()
	log.Debug().Any("config", config).Msg("")

	SetupMetrics()
	go Metrics(config.AddrPort)

	Subscribe(*config)
}
