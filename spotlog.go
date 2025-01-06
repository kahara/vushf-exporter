package main

import (
	"bytes"
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"math/rand"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"
)

var (
	Spots      []*Payload
	SpotLock   sync.Mutex
	Streamers  map[uint64]chan Payload
	StreamLock sync.Mutex
)

type Filter struct {
	Enabled  bool
	Locator  string
	Callsign string
	Bands    []string
	Modes    []string
}

var (
	pageTemplate     *template.Template
	tablerowTemplate *template.Template
)

func Spotlog(config Config, spots <-chan Payload) {
	Streamers = make(map[uint64]chan Payload)
	go serve(config)

	for spot := range spots {
		log.Debug().Any("payload", spot).Msg("Spotlogging")
		SpotLock.Lock()
		Spots = append(Spots, &spot)
		SpotLock.Unlock()

		StreamLock.Lock()
		for key, _ := range Streamers {
			Streamers[key] <- spot
		}
		StreamLock.Unlock()

		// Prune occasionally
		if rand.Float32() > 0.98 {
			log.Debug().Dur("retention", config.SpotlogRetention).Msg("Pruning spotlog spots")
			retainedSpots := make([]*Payload, 0)
			cutoff := uint64(time.Now().UTC().Add(-config.SpotlogRetention).Unix())
			SpotLock.Lock()
			for _, retained := range Spots {
				if retained.Time >= cutoff {
					retainedSpots = append(retainedSpots, retained)
				}
			}
			copy(Spots, retainedSpots)
			SpotLock.Unlock()
		}
	}
}

func GetSpots() []*Payload {
	SpotLock.Lock()
	sort.Slice(Spots, func(i, j int) bool {
		return Spots[i].Time < Spots[j].Time
	})
	spots := make([]*Payload, len(Spots))
	copy(spots, Spots)
	SpotLock.Unlock()

	return spots
}

func serve(config Config) {
	var err error

	pageTemplate, err = template.New("page").Parse(pageHtml)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse page template")
	}

	tablerowTemplate, err = template.New("tablerow").Parse(tablerowHtml)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse tablerow template")
	}

	log.Debug().Any("page", pageTemplate).Any("tablerow", tablerowTemplate).Msg("Templates parsed")

	spotlogMux := http.NewServeMux()
	spotlogMux.HandleFunc("GET /", pageHandler(config))
	spotlogMux.HandleFunc("GET /stream/", streamHandler)
	log.Fatal().Err(http.ListenAndServe(config.SpotlogAddrPort, spotlogMux)).Send()
}

func pageHandler(config Config) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {

		log.Debug().Msg("Serving a page")

		filter := NewFilter(request)
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")

		var tablerows []string
		for _, spot := range slices.Backward(GetSpots()) {
			if filter.Enabled && !filterSpot(filter, *spot) {
				continue
			}
			var row bytes.Buffer
			if err := tablerowTemplate.Execute(&row, spot); err != nil {
				log.Fatal().Err(err).Msg("Could not render table row template")
			}
			tablerows = append(tablerows, row.String())
		}

		if err := pageTemplate.Execute(writer, struct {
			Config    Config
			Filter    Filter
			Tablerows []string
		}{
			Config:    config,
			Filter:    filter,
			Tablerows: tablerows,
		}); err != nil {
			log.Fatal().Err(err).Msg("Failed to render page template")
		}
	}
}

func streamHandler(writer http.ResponseWriter, request *http.Request) {
	log.Debug().Msg("Streaming spots")
	filter := NewFilter(request)

	id := rand.Uint64()
	spots := make(chan Payload, 100)

	StreamLock.Lock()
	Streamers[id] = spots
	StreamLock.Unlock()

	defer func() {
		StreamLock.Lock()
		delete(Streamers, id)
		StreamLock.Unlock()
	}()

	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	io.WriteString(writer, ": keepalive\n\n")
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}

	keepalive := time.NewTicker(25 * time.Second)

	for {
		select {
		case <-keepalive.C:
			io.WriteString(writer, ": keepalive\n\n")
			if flusher, ok := writer.(http.Flusher); ok {
				flusher.Flush()
			}
		case spot := <-Streamers[id]:
			if filter.Enabled && !filterSpot(filter, spot) {
				continue
			}
			var row bytes.Buffer
			if err := tablerowTemplate.Execute(&row, spot); err != nil {
				log.Fatal().Err(err).Msg("Could not render table row template")
			}
			io.WriteString(writer, fmt.Sprintf("data: %s\n\n", row.String()))
			if flusher, ok := writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
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

func filterSpot(filter Filter, spot Payload) bool {
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

const pageHtml = `<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<title>Spotlog</title>
		<style>
		body {
			font-family: monospace;
		}
		table {
			border: 1px solid #999999;
			table-layout: auto;
			border-collapse: collapse;
			border-spacing: 1px;
			text-align: left;
		}
		tr:nth-child(even) {
			background-color: #eeeeee;
		}
		tbody tr:hover {
			background: #dddddd;
		}
		th {
			border: 1px solid #999999;
			color: #000000;
			padding: 5px;
		}
		td {
			border: 1px solid #999999;
			color: #000000;
			padding: 5px;
		}
		</style>
	</head>
	<body>
		<p>
			Data sourced from N1DQ's <a href="https://pskreporter.info/">PSK Reporter</a>
			over M0LTE's <a href="http://mqtt.pskreporter.info/">MQTT feed</a>. Thanks!
			This is <a href="https://github.com/kahara/vushf-exporter">kahara/vushf-exporter</a> by OH2EWL.
		</p>

		<p>
			Recording country
			<strong>{{.Config.Country}}</strong>
			on
			{{range .Config.Bands}}
			<strong>{{.}}</strong>
			{{end}}
			with
			<strong>{{.Config.SpotlogRetention.String}}</strong>
			retention
		</p>

		<details style="margin-bottom: 0.65em;">
			<summary>Parameters</summary>
			<table>
				<thead>
					<tr><th>Parameter</th><th>Example</th><th>Note</th></tr>
				</thead>
				<tbody>
					<tr><td>bands</td><td>bands=6m,4m,2m,70cm,23cm</td><td>Match list exactly</td></tr>
					<tr><td>modes</td><td>modes=FT8,FT4</td><td>Match list exactly</td></tr>
					<tr><td>locator</td><td>locator=KP20</td><td>Match prefix</td></tr>
					<tr><td>callsign</td><td>callsign=OH2</td><td>Match prefix</td></tr>
				</tbody>
			</table>
		</details>

		{{if .Filter.Enabled}}
		<p>
			Filter
			<strong>
			{{range .Filter.Bands}}
			{{.}}
			{{end}}
			{{range .Filter.Modes}}
			{{.}}
			{{end}}
			{{.Filter.Locator}}
			{{.Filter.Callsign}}
			</strong>
		<p>
		{{end}}

		<table>
			<thead>
				<tr>
					<th>Sequence</th>
					<th>Time</th>
					<th>Band</th>
					<th>Mode</th>
					<th>Report</th>
					<th>Distance</th>
					<th>Frequency</th>
					<th>Tx callsign</th>
					<th>Tx locator</th>
					<th>Tx country</th>
					<th>Rx callsign</th>
					<th>Rx locator</th>
					<th>Rx country</th>
				</tr>
			</thead>
			<tbody id="spots">
				{{range .Tablerows}}{{ . }}{{end}}
			</tbody>
		</table>

		<script>
		const table = document.getElementById('spots');
		const spots = new EventSource('/stream/' + window.location.search);
		spots.onmessage = function(spot) {
			console.log(spot);
			const template = document.createElement('template');
			template.innerHTML = spot.data;
			table.prepend(template.content.firstElementChild);
		};
		</script>
	</body>
</html>
`

const tablerowHtml = `<tr><td>{{.SequenceHex}}</td><td>{{.FormattedTime}}</td><td>{{.Band}}</td><td>{{.Mode}}</td><td>{{.Report}}</td><td>{{.Distance}}</td><td>{{.Mhz}}</td><td>{{.SenderCallsign}}</td><td>{{.SenderLocator}}</td><td>{{.SenderCountry}}</td><td>{{.ReceiverCallsign}}</td><td>{{.ReceiverLocator}}</td><td>{{.ReceiverCountry}}</td></tr>`
