package main

import (
	"github.com/rs/zerolog/log"
	"net/http"
	"slices"
	"strings"
)

const (
	MaxModeNameLength = 8
	MaxModeCount      = 8
	MaxLocatorLength  = 16
	MaxCallsignLength = 16
)

type Filter struct {
	Enabled  bool
	Locator  string
	Callsign string
	Bands    []string
	Modes    []string
}

func NewFilter(config Config, request *http.Request) Filter {
	filter := Filter{
		Enabled: false,
		Bands: func() []string {
			var bands []string
			for _, band := range strings.Split(request.URL.Query().Get("bands"), ",") {
				if slices.Contains(config.Bands, band) && !slices.Contains(bands, band) {
					bands = append(bands, band)
				}
			}
			return bands
		}(),
		Modes: func() []string {
			var modes []string
			for _, mode := range strings.Split(request.URL.Query().Get("modes"), ",") {
				if mode != "" && len(mode) <= MaxModeNameLength && !slices.Contains(modes, mode) {
					modes = append(modes, mode)
				}
			}
			return modes[:min(len(modes), MaxModeCount)]
		}(),
		Locator: func() string {
			locator := request.URL.Query().Get("locator")
			return locator[:min(len(locator), MaxLocatorLength)]
		}(),
		Callsign: func() string {
			callsign := request.URL.Query().Get("callsign")
			return callsign[:min(len(callsign), MaxCallsignLength)]
		}(),
	}

	if filter.Bands != nil || filter.Modes != nil || filter.Locator != "" || filter.Callsign != "" {
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
