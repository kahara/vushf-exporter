package main

import (
	"github.com/rs/zerolog/log"
	"net/http"
	"slices"
	"strings"
)

type Filter struct {
	Enabled  bool
	Locator  string
	Callsign string
	Bands    []string
	Modes    []string
}

func NewFilter(request *http.Request) Filter {
	filter := Filter{
		Enabled:  false,
		Locator:  request.URL.Query().Get("locator"),
		Callsign: request.URL.Query().Get("callsign"),
		Bands:    strings.Split(request.URL.Query().Get("bands"), ","),
		Modes:    strings.Split(request.URL.Query().Get("modes"), ","),
	}
	if filter.Locator != "" || filter.Callsign != "" {
		filter.Enabled = true
	}
	if filter.Bands[0] == "" {
		filter.Bands = nil
	} else {
		filter.Enabled = true
	}
	if filter.Modes[0] == "" {
		filter.Modes = nil
	} else {
		filter.Enabled = true
	}

	log.Debug().Any("filter", filter).Msg("Filter filters")

	return filter
}

func (filter *Filter) filter(spot Payload) bool {
	// Locator
	if filter.Locator != "" && !(strings.HasPrefix(spot.SenderLocator, filter.Locator) || strings.HasPrefix(spot.ReceiverLocator, filter.Locator)) {
		return false
	}

	// Callsign
	if filter.Callsign != "" && !(strings.HasPrefix(spot.SenderCallsign, filter.Callsign) || strings.HasPrefix(spot.ReceiverCallsign, filter.Callsign)) {
		return false
	}

	// Band
	if len(filter.Bands) > 0 && !slices.Contains(filter.Bands, spot.Band) {
		return false
	}

	// Mode
	if len(filter.Modes) > 0 && !slices.Contains(filter.Modes, spot.Mode) {
		return false
	}

	return true
}
