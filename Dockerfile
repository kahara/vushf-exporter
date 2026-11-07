FROM golang:1.26.1-trixie AS build

RUN mkdir /workdir
COPY go.* /workdir/
COPY *.go /workdir/

WORKDIR /workdir
RUN go build -o vushf-exporter .

FROM gcr.io/distroless/base-debian13 AS production

COPY --from=build /workdir/vushf-exporter /
COPY favicon.ico /

CMD ["/vushf-exporter"]
