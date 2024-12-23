package main

import "github.com/rs/zerolog/log"

func Spotlog(addrPort string, spots <-chan Payload) {
	for payload := range spots {
		log.Debug().Any("payload", payload).Msg("Spotlogging")
	}
}
