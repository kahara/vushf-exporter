package main

import (
	"bytes"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"slices"
	"strings"
	"text/template"
)

func Spotlog(addrPort string, spots <-chan Payload) {
	go serve(addrPort)

	for spot := range spots {
		log.Debug().Any("payload", spot).Msg("Spotlogging")
		Spots = append(Spots, spot)
	}
}

var Spots []Payload

type Filter struct {
	Locator  string
	Callsign string
	Bands    []string
	Modes    []string
}

var (
	pageTemplate     *template.Template
	tablerowTemplate *template.Template
)

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
	spotlogMux.HandleFunc("/", spotlogHandler)
	log.Fatal().Err(http.ListenAndServe(":8080", spotlogMux)).Send()
}

func spotlogHandler(writer http.ResponseWriter, request *http.Request) {
	filter := Filter{
		Locator:  request.URL.Query().Get("locator"),
		Callsign: request.URL.Query().Get("callsign"),
		Bands:    strings.Split(request.URL.Query().Get("bands"), ","),
		Modes:    strings.Split(request.URL.Query().Get("modes"), ","),
	}
	if filter.Bands[0] == "" {
		filter.Bands = nil
	}
	if filter.Modes[0] == "" {
		filter.Modes = nil
	}

	log.Debug().Any("filter", filter).Msg("Filter filters")

	if request.URL.Path == "/" {
		log.Debug().Msg("Serving a page")
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")

		var tablerows []string
		for _, spot := range Spots {
			if !filterSpot(filter, spot) {
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
	} else if request.URL.Path == "/stream" || request.URL.Path == "/stream/" {
		log.Debug().Msg("Streaming spots")
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Header().Set("Cache-Control", "no-cache")
		writer.Header().Set("Connection", "keep-alive")
		// "data: xxx\n\n"
	} else {
		writer.WriteHeader(http.StatusTeapot)
		io.WriteString(writer, "¯\\_(?)_/¯\n")
	}
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
					<th>UTC</th>
					<th>Frequency</th>
					<th>Band</th>
					<th>Mode</th>
					<th>Report</th>
					<th>Tx callsign</th>
					<th>Tx locator</th>
					<th>Tx country</th>
					<th>Rx callsign</th>
					<th>Rx locator</th>
					<th>Rx country</th>
				</tr>
			</thead>
			<tbody>
				{{range .Tablerows}}{{ . }}{{end}}
			</tbody>
	</body>
</html>
`

const tablerowHtml = `<tr><td>{{.RFC3339}}</td><td>{{.Mhz}}</td><td>{{.Band}}</td><td>{{.Mode}}</td><td>{{.Report}}</td><td>{{.SenderCallsign}}</td><td>{{.SenderLocator}}</td><td>{{.SenderCountry}}</td><td>{{.ReceiverCallsign}}</td><td>{{.ReceiverLocator}}</td><td>{{.ReceiverCountry}}</td></tr>`
