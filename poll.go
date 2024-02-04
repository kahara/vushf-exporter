package main

import (
	"encoding/json"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
	"strconv"
	"time"
)

type Payload struct {
	SequenceNumber   uint64 `json:"sq"`
	Frequency        int    `json:"f"`
	Mode             string `json:"md"`
	Report           int    `json:"rp"`
	Time             uint64 `json:"t"`
	SenderCallsign   string `json:"sc"`
	SenderLocator    string `json:"sl"`
	ReceiverCallsign string `json:"rc"`
	ReceiverLocator  string `json:"rl"`
	SenderCountry    int    `json:"sa"`
	ReceiverCountry  int    `json:"ra"`
	Band             string `json:"b"`
}

func Poll(server string, name string, topic string, country int) {

	opts := mqtt.NewClientOptions()
	opts.AddBroker(server)
	opts.SetKeepAlive(2 * time.Second)
	opts.SetPingTimeout(1 * time.Second)
	client := mqtt.NewClient(opts)

	for {
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			log.Err(token.Error()).Msg("")
			time.Sleep(time.Duration(time.Second))
			continue
		}

		if token := client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
			var payload Payload
			if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
				log.Err(err).Msg("")
				return
			}

			log.Debug().Str("topic", msg.Topic()).Any("payload", payload).Msg("received")
			if payload.SenderCountry == payload.ReceiverCountry {
				log.Debug().Msg("Skipping message within same country")
				return
			} else if payload.SenderCountry == country {
				sent_metric.WithLabelValues(strconv.Itoa(country), payload.Band).Inc()
			} else if payload.ReceiverCountry == country {
				received_metric.WithLabelValues(strconv.Itoa(country), payload.Band).Inc()
			} else {
				// Not sure how we got here
				log.Debug().Msg("No country matches")
			}
		}); token.Wait() && token.Error() != nil {
			log.Err(token.Error()).Msg("")
			time.Sleep(time.Duration(time.Second))
			continue
		}

		for {
			time.Sleep(time.Duration(time.Second))
		}
	}
}
