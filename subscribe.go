package main

import (
	"encoding/json"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
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
	opts.AddBroker(config.Broker)
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
		log.Info().Str("server", config.Broker).Msg("Connecting")

		token := client.SubscribeMultiple(topics, func(client mqtt.Client, msg mqtt.Message) {
			var payload Payload
			if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
				log.Err(err).Msg("Payload unmarshalling failed")
				return
			}

			if payload.SenderCountry == payload.ReceiverCountry {
				log.Debug().Str("topic", msg.Topic()).Any("payload", payload).Msg("Recording message within same country")
				local_metric.WithLabelValues(strconv.Itoa(config.Country), payload.Band).Inc()
			} else if payload.SenderCountry == config.Country {
				log.Debug().Str("topic", msg.Topic()).Any("payload", payload).Msg("Recording message sent from target country")
				sent_metric.WithLabelValues(strconv.Itoa(config.Country), payload.Band).Inc()
			} else if payload.ReceiverCountry == config.Country {
				log.Debug().Str("topic", msg.Topic()).Any("payload", payload).Msg("Recording message received in target country")
				received_metric.WithLabelValues(strconv.Itoa(config.Country), payload.Band).Inc()
			} else {
				// Not sure how we got here
				log.Debug().Str("topic", msg.Topic()).Any("payload", payload).Msg("No country matches, skipping")
			}
		})

		go func() {
			<-token.Done()
			//_ = token.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
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

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Err(token.Error()).Msg("")
		time.Sleep(time.Duration(time.Second))
	}
	log.Info().Str("server", config.Broker).Msg("Connected")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	signal.Notify(sig, syscall.SIGTERM)
	log.Info().Any("signal", <-sig).Msg("Signal caught, exiting")
	client.Disconnect(1000)
}
