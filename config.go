package main

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBands            = "6m,4m,2m,70cm,23cm"
	DefaultCountry          = 224 // Finland; see https://www.adif.org/304/ADIF_304.htm#Country_Codes
	DefaultBroker           = "mqtt.pskreporter.info:1883"
	DefaultMetricsAddrPort  = ":9108"
	DefaultSpotlogAddrPort  = ":8080"
	DefaultSpotlogRetention = time.Duration(time.Hour * 24)
)

type Config struct {
	Broker           string
	Bands            []string
	Country          int
	Topics           []string
	MetricsAddrPort  string
	SpotlogAddrPort  string
	SpotlogRetention time.Duration
}

func NewConfig() *Config {
	var config Config

	// Bands
	bands := os.Getenv("BANDS")
	if bands == "" {
		config.Bands = strings.Split(DefaultBands, ",")
	} else {
		config.Bands = strings.Split(bands, ",")
	}

	// Country
	country := os.Getenv("COUNTRY")
	if country == "" {
		config.Country = DefaultCountry
	} else {
		c, _ := strconv.Atoi(country)
		config.Country = c
	}

	// MQTT topics
	for _, band := range config.Bands {
		config.Topics = append(config.Topics, fmt.Sprintf("pskr/filter/v2/%s/+/+/+/+/+/%d/+", band, config.Country))
		config.Topics = append(config.Topics, fmt.Sprintf("pskr/filter/v2/%s/+/+/+/+/+/+/%d", band, config.Country))
	}

	// MQTT broker
	mqttServer := os.Getenv("BROKER")
	if mqttServer == "" {
		config.Broker = DefaultBroker
	} else {
		config.Broker = mqttServer
	}

	// Metrics' address
	metricsAddrPort := os.Getenv("METRICS_ADDRPORT")
	if metricsAddrPort == "" {
		config.MetricsAddrPort = DefaultMetricsAddrPort
	} else {
		config.MetricsAddrPort = metricsAddrPort
	}

	// Spotlog address
	spotlogAddrPort := os.Getenv("SPOTLOG_ADDRPORT")
	if spotlogAddrPort == "" {
		config.SpotlogAddrPort = DefaultSpotlogAddrPort
	} else {
		config.SpotlogAddrPort = spotlogAddrPort
	}

	// Spotlog retention
	spotlogRetention := os.Getenv("SPOTLOG_RETENTION")
	if spotlogRetention == "" {
		config.SpotlogRetention = DefaultSpotlogRetention
	} else {
		if duration, err := time.ParseDuration(spotlogRetention); err != nil {
			log.Fatal().Err(err).Str("retention", spotlogRetention).Msg("Could not parse SPOTLOG_RETENTION")
		} else {
			config.SpotlogRetention = duration
		}
	}

	return &config
}
