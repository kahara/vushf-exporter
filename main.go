package main

import (
	"github.com/rs/zerolog/log"
	"time"
)

func main() {
	var config = NewConfig()

	log.Debug().Any("config", config).Msg("")

	SetupMetrics(config.Topics)
	go Metrics(config.MetricAddr)

	for name, topic := range config.Topics {
		go Poll(config.MqttServer, name, topic, config.TargetCountry)
	}

	for {
		time.Sleep(time.Second)
	}
}
