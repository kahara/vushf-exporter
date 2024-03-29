package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultBands    = "6m,4m,2m,70cm,23cm"
	DefaultCountry  = 224 // Finland; see https://www.adif.org/304/ADIF_304.htm#Country_Codes
	DefaultBroker   = "mqtt.pskreporter.info:1883"
	DefaultAddrPort = ":9108"
)

type Config struct {
	Broker   string
	Bands    []string
	Country  int
	Topics   []string
	AddrPort string
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
	targetCountry := os.Getenv("COUNTRY")
	if targetCountry == "" {
		config.Country = DefaultCountry
	} else {
		c, _ := strconv.Atoi(targetCountry)
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
	addrPort := os.Getenv("ADDRPORT")
	if addrPort == "" {
		config.AddrPort = DefaultAddrPort
	} else {
		config.AddrPort = addrPort
	}

	return &config
}
