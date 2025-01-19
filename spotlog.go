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
	"sync"
	"text/template"
	"time"
)

type Streamer struct {
	Keepalive time.Time
	Spots     chan *Payload
}

var (
	Spots            []*Payload
	SpotLock         sync.Mutex
	Streamers        map[uint64]*Streamer
	StreamLock       sync.Mutex
	pageTemplate     *template.Template
	tablerowTemplate *template.Template
)

func Spotlog(config Config, spots <-chan *Payload) {
	Streamers = make(map[uint64]*Streamer)
	go serve(config)

	for spot := range spots {
		log.Debug().Any("payload", spot).Msg("Spotlogging")
		SpotLock.Lock()
		Spots = append(Spots, spot)
		SpotLock.Unlock()

		StreamLock.Lock()
		log.Debug().Int("streamers", len(Streamers)).Msg("Feeding to streamers")
		now := time.Now()
		for key, _ := range Streamers {
			// This should not happen but perhaps a streamer is already gone, yet didn't clean up after itself
			if now.Sub(Streamers[key].Keepalive) > time.Minute {
				log.Debug().Uint64("id", key).Msg("Forcibly removing streamer")
				delete(Streamers, key)
				continue
			}
			// Not a very likely occurrence, but guard against filled-up stream channels
			select {
			case Streamers[key].Spots <- spot:
			default:
			}
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
			Spots = make([]*Payload, len(retainedSpots))
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
	spotlogMux.HandleFunc("GET /favicon.ico", faviconHandler)
	spotlogMux.HandleFunc("GET /stream/", streamHandler(config))
	log.Fatal().Err(http.ListenAndServe(config.SpotlogAddrPort, spotlogMux)).Send()
}

func faviconHandler(writer http.ResponseWriter, request *http.Request) {
	http.ServeFile(writer, request, "favicon.ico")
}

func pageHandler(config Config) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {

		log.Debug().Msg("Serving a page")

		filter := NewFilter(config, request)
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")

		var tablerows []string
		for _, spot := range slices.Backward(GetSpots()) {
			if filter.Enabled && !filter.filter(*spot) {
				continue
			}
			var row bytes.Buffer
			if err := tablerowTemplate.Execute(&row, spot); err != nil {
				log.Error().Err(err).Msg("Could not render table row template")
			} else {
				tablerows = append(tablerows, row.String())
			}
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
			log.Error().Err(err).Msg("Failed to render page template")
		}
	}
}

func streamHandler(config Config) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		log.Debug().Msg("Streaming spots")
		filter := NewFilter(config, request)

		id := rand.Uint64()
		StreamLock.Lock()
		log.Debug().Uint64("id", id).Msg("Adding streamer")
		Streamers[id] = &Streamer{
			Keepalive: time.Now(),
			Spots:     make(chan *Payload, 1000),
		}
		StreamLock.Unlock()

		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Header().Set("Cache-Control", "no-cache")
		writer.Header().Set("Connection", "keep-alive")
		io.WriteString(writer, ": keepalive\n\n")
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}

		keepalive := time.NewTicker(25 * time.Second)
		update := time.NewTicker(333 * time.Millisecond)

		for {
			done := false
			select {
			case <-request.Context().Done():
				log.Debug().Uint64("id", id).Msg("Streamer is gone, removing")
				StreamLock.Lock()
				delete(Streamers, id)
				StreamLock.Unlock()
				done = true
				break
			case <-keepalive.C:
				StreamLock.Lock()
				Streamers[id].Keepalive = time.Now()
				StreamLock.Unlock()
				io.WriteString(writer, ": keepalive\n\n")
				if flusher, ok := writer.(http.Flusher); ok {
					flusher.Flush()
				}
			case <-update.C:
				var spots []*Payload

				StreamLock.Lock()
				for {
					updated := false
					select {
					case spot := <-Streamers[id].Spots:
						if filter.Enabled && !filter.filter(*spot) {
							continue
						}
						spots = append(spots, spot)
					default:
						updated = true
					}
					if updated {
						break
					}
				}
				StreamLock.Unlock()

				if len(spots) > 0 {
					for _, spot := range spots {
						var row bytes.Buffer
						if err := tablerowTemplate.Execute(&row, spot); err != nil {
							log.Error().Err(err).Msg("Could not render table row template")
						} else {
							io.WriteString(writer, fmt.Sprintf("data: %s\n\n", row.String()))
						}
					}
					if flusher, ok := writer.(http.Flusher); ok {
						flusher.Flush()
					}
				}
			}
			if done {
				break
			}
		}
	}
}

const pageHtml = `<!DOCTYPE html>
<html lang="en">
	<head>
		<meta charset="utf-8">
		<meta name="author" content="Joni OH2EWL">
		<meta name="description" content="Live view of PSK Reporter's spots from and to country {{.Config.Country}}">
		<title>
		Spotlog
		{{range .Filter.Bands}}
		{{.}}
		{{end}}
		{{range .Filter.Modes}}
		{{.}}
		{{end}}
		{{.Filter.Locator}}
		{{.Filter.Callsign}}
		</title>
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
			<p>
				Examples:
				<a href="/?bands=2m,70cm">?bands=2m,70cm</a>
				<a href="/?bands=2m,70cm&modes=FT8">?bands=2m,70cm&modes=FT8</a>
				<a href="/?modes=FT4,WSPR&locator=KP20&callsign=OH2">?modes=FT4,WSPR&locator=KP20&callsign=OH2</a>
				<a href="/?callsign=OH2EWL">?callsign=OH2EWL</a>
			</p>
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
					<th>Tx call</th>
					<th>locator</th>
					<th>country</th>
					<th>Rx call</th>
					<th>locator</th>
					<th>country</th>
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

const tablerowHtml = `<tr><td>{{.SequenceHex}}</td><td>{{.FormattedTime}}</td><td>{{.Band}}</td><td>{{.Mode}}</td><td style="text-align: center;">{{.Report}}</td><td style="text-align: right;">{{.Distance}}</td><td style="text-align: right;">{{printf "%.6f" .Mhz}}</td><td>{{.SenderCallsign}}</td><td>{{.SenderLocator}}</td><td>{{.SenderCountry}}</td><td>{{.ReceiverCallsign}}</td><td>{{.ReceiverLocator}}</td><td>{{.ReceiverCountry}}</td></tr>`
