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

func Subscribe(config Config) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.MqttServer)
	opts.SetKeepAlive(10 * time.Second)
	opts.SetPingTimeout(2 * time.Second)
	opts.SetOrderMatters(false)
	opts.SetConnectRetry(true)
	opts.SetAutoReconnect(true)

	topics := make(map[string]byte)
	for _, topic := range config.Topics {
		topics[topic] = 0
	}

	opts.OnConnect = func(client mqtt.Client) {
		token := client.SubscribeMultiple(topics, func(client mqtt.Client, msg mqtt.Message) {
			var payload Payload
			if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
				log.Err(err).Msg("")
				return
			}
			log.Debug().Str("topic", msg.Topic()).Any("payload", payload).Msg("received")

			if payload.SenderCountry == payload.ReceiverCountry {
				log.Debug().Msg("Skipping message within same country")
				return
			} else if payload.SenderCountry == config.TargetCountry {
				sent_metric.WithLabelValues(strconv.Itoa(config.TargetCountry), payload.Band).Inc()
			} else if payload.ReceiverCountry == config.TargetCountry {
				received_metric.WithLabelValues(strconv.Itoa(config.TargetCountry), payload.Band).Inc()
			} else {
				// Not sure how we got here
				log.Debug().Msg("No country matches")
			}
		})

		go func() {
			_ = token.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
			if token.Error() != nil {
				log.Err(token.Error()).Msg("Error subscribing")
			} else {
				log.Info().Any("topics", topics).Msg("Subscribed")
			}
		}()
	}

	opts.OnConnectionLost = func(cl mqtt.Client, err error) {
		log.Err(err).Msg("Connection lost")
	}

	opts.OnReconnecting = func(mqtt.Client, *mqtt.ClientOptions) {
		log.Info().Msg("Reconnecting")
	}

	client := mqtt.NewClient(opts)

	for {
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			log.Err(token.Error()).Msg("")
			time.Sleep(time.Duration(time.Second))
			continue
		}
	}
}
