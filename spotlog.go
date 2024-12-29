package main

import (
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"strings"
	"text/template"
)

func Spotlog(addrPort string, spots <-chan Payload) {
	go serve(addrPort)

	for payload := range spots {
		log.Debug().Any("payload", payload).Msg("Spotlogging")
	}
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
	spotlogMux.HandleFunc("/", spotlogHandler)
	log.Fatal().Err(http.ListenAndServe(":8080", spotlogMux)).Send()
}

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

func spotlogHandler(writer http.ResponseWriter, request *http.Request) {
	filter := Filter{
		Locator:  request.URL.Query().Get("locator"),
		Callsign: request.URL.Query().Get("callsign"),
		Bands:    strings.Split(request.URL.Query().Get("bands"), ","),
		Modes:    strings.Split(request.URL.Query().Get("modes"), ","),
	}

	log.Debug().Any("filter", filter).Msg("Filter filters")

	if request.URL.Path == "/" {
		log.Debug().Msg("Serving a page")
		if err := pageTemplate.Execute(writer, struct {
			Filter    Filter
			Tablerows []string
		}{
			Filter:    filter,
			Tablerows: []string{"foo", "bar", "baz"},
		}); err != nil {
			log.Fatal().Err(err).Msg("Failed to render page template")
		}
	} else if request.URL.Path == "/stream" || request.URL.Path == "/stream/" {
		log.Debug().Msg("Streaming spots")

	} else {
		writer.WriteHeader(http.StatusTeapot)
		io.WriteString(writer, "¯\\_(?)_/¯\n")
	}
}

const pageHtml = `<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<title>Spotlog</title>
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
					<th>Time</th>
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

const tablerowHtml = `<tr><td>{{.Time}}</td><td>{{.Frequency}}</td><td>{{.Band}}</td><td>{{.Mode}}</td><td>{{.Report}}</td><td>{{.SenderCallsign}}</td><td>{{.SenderLocator}}</td><td>{{.SenderCountry}}</td><td>{{.ReceiverCallsign}}</td><td>{{.ReceiverLocator}}</td><td>{{.ReceiverCountry}}</td></tr>`
