package main

import (
	"encoding/json"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type Payload struct {
	SequenceNumber   uint64  `json:"sq"`
	Frequency        int     `json:"f"`
	Mhz              float64 `json:"mhz,omitempty"`
	Mode             string  `json:"md"`
	Report           int     `json:"rp"`
	Time             uint64  `json:"t"`
	RFC3339          string  `json:"utc,omitempty"`
	SenderCallsign   string  `json:"sc"`
	SenderLocator    string  `json:"sl"`
	ReceiverCallsign string  `json:"rc"`
	ReceiverLocator  string  `json:"rl"`
	SenderCountry    int     `json:"sa"`
	ReceiverCountry  int     `json:"ra"`
	Band             string  `json:"b"`
}

var (
	seenMessages map[mqtt.Message]time.Time
	seenMutex    sync.Mutex
)

func Subscribe(config Config, spots chan<- Payload) {
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

	seenMessages = make(map[mqtt.Message]time.Time)

	opts.OnConnect = func(client mqtt.Client) {
		log.Info().Str("server", config.Broker).Msg("Subscribing")

		token := client.SubscribeMultiple(topics, func(client mqtt.Client, message mqtt.Message) {
			// Keep track of duplicates
			seenMutex.Lock()
			if _, seen := seenMessages[message]; seen {
				seenMutex.Unlock()
				prune()
				return
			}
			seenMessages[message] = time.Now()
			seenMutex.Unlock()

			var payload Payload
			if err := json.Unmarshal(message.Payload(), &payload); err != nil {
				log.Err(err).Msg("Payload unmarshalling failed")
				return
			}
			payload.RFC3339 = time.Unix(int64(payload.Time), 0).UTC().Format(time.RFC3339)
			payload.Mhz = float64(payload.Frequency) / 1000000

			spots <- payload

			if payload.SenderCountry == payload.ReceiverCountry {
				log.Debug().Str("topic", message.Topic()).Any("payload", payload).Msg("Recording message within same country")
				local_metric.WithLabelValues(strconv.Itoa(config.Country), payload.Band).Inc()
			} else if payload.SenderCountry == config.Country {
				log.Debug().Str("topic", message.Topic()).Any("payload", payload).Msg("Recording message sent from target country")
				sent_metric.WithLabelValues(strconv.Itoa(config.Country), payload.Band).Inc()
			} else if payload.ReceiverCountry == config.Country {
				log.Debug().Str("topic", message.Topic()).Any("payload", payload).Msg("Recording message received in target country")
				received_metric.WithLabelValues(strconv.Itoa(config.Country), payload.Band).Inc()
			} else {
				// Not sure how we got here
				log.Debug().Str("topic", message.Topic()).Any("payload", payload).Msg("No country matches, skipping")
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

// Clean up already-seen messages, occasionally
func prune() {
	if rand.Float32() < 0.9 {
		return
	}
	log.Debug().Any("length", len(seenMessages)).Msg("Start pruning")

	count := 0
	now := time.Now()
	seenMutex.Lock()
	defer seenMutex.Unlock()
	for key, seen := range seenMessages {
		if now.Sub(seen) > time.Duration(time.Minute) {
			delete(seenMessages, key)
			count += 1
		}
	}

	log.Debug().Any("length", len(seenMessages)).Int("pruned", count).Msg("Done pruning")
}
