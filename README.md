# V/U/SHF exporter

Consume V/U/SHF spots from [mqtt.pskreporter.info](http://mqtt.pskreporter.info/)
and aggregate them for Prometheus' consumption.

In this context, "V/U/SHF" is to be interpreted as 6m, 4m, 2m, 70cm, and 23cm,
and right now we're only interested in spot counts in the short term, like a
few hours or so. No further filtering is performed, the only label added is the
country, and all modes are considered. Given that there are five bands (by default),
and we're looking at two directions for each, we'd have a total of ten counters
for Prometheus to consume. For example:

```
pskreporter_spots_sent_total{country="224", band="6m"} 5541
pskreporter_spots_received_total{country="224", band="6m"} 31233
pskreporter_spots_sent_total{country="224", band="4m"} 2204
pskreporter_spots_received_total{country="224", band="4m"} 7767
pskreporter_spots_sent_total{country="224", band="2m"} 10374
pskreporter_spots_received_total{country="224", band="2m"} 11474
pskreporter_spots_sent_total{country="224", band="70cm"} 26363
pskreporter_spots_received_total{country="224", band="70cm"} 11786
pskreporter_spots_sent_total{country="224", band="23cm"} 23575
pskreporter_spots_received_total{country="224", band="23cm"} 20196
```

"Local" spots, i.e. ones which are sent and received in the same country, are skipped.

## Configuration

Default are as follows:

* MQTT_SERVER `mqtt.pskreporter.info:1883`
* TARGET_COUNTRY `224`
* BANDS `6m,4m,2m,70cm,23cm`
* METRICS_ADDR `:9108`
