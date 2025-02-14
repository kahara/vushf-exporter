FROM golang:1.23.4-bullseye AS build

RUN mkdir /workdir
COPY go.* /workdir/
COPY *.go /workdir/

WORKDIR /workdir
RUN go build -o vushf-exporter .

FROM gcr.io/distroless/base-debian12 AS production

COPY --from=build /workdir/vushf-exporter /
COPY favicon.ico /

CMD ["/vushf-exporter"]
