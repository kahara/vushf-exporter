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

func Spotlog(addrPort string, spots <-chan Payload) {
	Streamers = make(map[uint64]chan Payload)
	go serve(addrPort)

	for spot := range spots {
		log.Debug().Any("payload", spot).Msg("Spotlogging")
		SpotLock.Lock()
		Spots = append(Spots, &spot)
		SpotLock.Unlock()

		StreamLock.Lock()
		for _, streamer := range Streamers {
			streamer <- spot
		}
		StreamLock.Unlock()
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

func serve(addrPort string) {
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
	spotlogMux.HandleFunc("/", pageHandler)
	spotlogMux.HandleFunc("/stream", streamHandler)
	log.Fatal().Err(http.ListenAndServe(":8080", spotlogMux)).Send()
}

func pageHandler(writer http.ResponseWriter, request *http.Request) {
	log.Debug().Msg("Serving a page")

	filter := NewFilter(request)
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")

	var tablerows []string
	for _, spot := range slices.Backward(GetSpots()) {
		log.Debug().Any("spot", spot).Send()
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
		Filter    Filter
		Tablerows []string
	}{
		Filter:    filter,
		Tablerows: tablerows,
	}); err != nil {
		log.Fatal().Err(err).Msg("Failed to render page template")
	}
}

func streamHandler(writer http.ResponseWriter, request *http.Request) {
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

	log.Debug().Msg("Streaming spots")
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")

	keepalive := time.NewTicker(25 * time.Second)

	for {
		select {
		case <-keepalive.C:
			io.WriteString(writer, ": keepalive\n\n")
		case spot := <-Streamers[id]:
			var row bytes.Buffer
			if err := tablerowTemplate.Execute(&row, spot); err != nil {
				log.Fatal().Err(err).Msg("Could not render table row template")
			}
			io.WriteString(writer, fmt.Sprintf("data: %s\n\n", row.String()))
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
			{{.Filter.Locator}}
			{{.Filter.Callsign}}
			{{range .Filter.Bands}}{{.}} {{end}}
			{{range .Filter.Modes}}{{.}} {{end}}
		</p>
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

		<script>
		const table = document.getElementById('spots');
		const spots = new EventSource('/stream');
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

const tablerowHtml = `<tr><td>{{.SequenceHex}}</td><td>{{.RFC3339}}</td><td>{{.Band}}</td><td>{{.Mode}}</td><td>{{.Report}}</td><td>{{.Distance}}</td><td>{{.Mhz}}</td><td>{{.SenderCallsign}}</td><td>{{.SenderLocator}}</td><td>{{.SenderCountry}}</td><td>{{.ReceiverCallsign}}</td><td>{{.ReceiverLocator}}</td><td>{{.ReceiverCountry}}</td></tr>`
