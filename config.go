package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultBands         = "6m,4m,2m,70cm,23cm"
	DefaultTargetCountry = 224 // Finland; see https://www.adif.org/304/ADIF_304.htm#Country_Codes
	DefaultMqttServer    = "mqtt.pskreporter.info:1883"
	DefaultMetricAddr    = ":9108"
)

type Config struct {
	Bands         []string
	TargetCountry int
	Topics        map[string]string
	MqttServer    string
	MetricAddr    string
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

	// Target country
	targetCountry := os.Getenv("TARGET_COUNTRY")
	if targetCountry == "" {
		config.TargetCountry = DefaultTargetCountry
	} else {
		c, _ := strconv.Atoi(targetCountry)
		config.TargetCountry = c
	}

	// MQTT topics
	config.Topics = make(map[string]string)
	for _, band := range config.Bands {
		config.Topics[fmt.Sprintf("%s_sent_total", band)] = fmt.Sprintf("pskr/filter/v2/%s/+/+/+/+/+/%d/+", band, config.TargetCountry)
		config.Topics[fmt.Sprintf("%s_received_total", band)] = fmt.Sprintf("pskr/filter/v2/%s/+/+/+/+/+/+/%d", band, config.TargetCountry)
	}

	// MQTT Server
	mqttServer := os.Getenv("MQTT_SERVER")
	if mqttServer == "" {
		config.MqttServer = DefaultMqttServer
	} else {
		config.MqttServer = mqttServer
	}

	// Metrics' address
	metricAddr := os.Getenv("METRICS_ADDR")
	if metricAddr == "" {
		config.MetricAddr = DefaultMetricAddr
	} else {
		config.MetricAddr = metricAddr
	}

	return &config
}
