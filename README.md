# otel-storjstats-receiver

An OpenTelemetry Collector receiver that ingests admproto-encoded UDP metric
packets (the wire format Storj node software emits, historically consumed by
[statreceiver]) and forwards a whitelisted subset as OTel metrics.

## Usage

Build the receiver into a custom collector distribution with
[`ocb`](https://opentelemetry.io/docs/collector/custom-collector/) by adding it
to the builder manifest:

```yaml
receivers:
  - gomod: github.com/elek/otel-storjstats-receiver v0.0.0
```

Then configure a pipeline:

```yaml
receivers:
  storjstats:
    endpoint: 0.0.0.0:9000
    read_buffer_size: 10240        # optional, default 10240
    include:
      - name: pieces_writer
        tags: { size: "2m" }
        fields: [ravg]
      - name: upload_success_size_bytes
        fields: [count, sum]
      - name: download_success_size_bytes
        fields: [count, sum]
      - name: upload_success_duration_ns
        fields: [ravg, count, r50, r99]
      - name: download_success_duration_ns
        fields: [ravg, count, r50, r99]
      - name: upload_success_count
        fields: [value]
      - name: download_success_count
        fields: [value]
      - name: version_info
      - name: hashstore.*
      - name: blobs_usage.*
      - name: satellite_usage.*
      - name: used_space.*
      - name: migration_status
        fields: [active]
      - name: migration_progress
        fields: [enqueued, processed, successes, errors, remaining_directories]
      - name: queue
        fields: [length]

service:
  pipelines:
    metrics:
      receivers: [storjstats]
      exporters: [debug]
```

## Include rules

Each rule is an AND over three matchers:

| Field | Meaning |
|---|---|
| `name` | Go regex matched against the metric's base name (the part before the first comma). Auto-anchored — write `hashstore.*`, not `^hashstore.*$`. |
| `tags` | Map of tag key=value pairs that must all be present on the sample. Extra tags are allowed. |
| `fields` | List of allowed field suffixes (the part after the space in the stat key, e.g. `count`, `ravg`). Empty = any field allowed. |
| `type` | Optional metric-type override: `gauge` or `sum`. If unset, per-field heuristic runs. |

Rules are evaluated in declaration order — first match wins. Unmatched
samples are silently dropped.

## Metric type heuristic

Applied per-field when a rule does not set `type:`:

- `count`, `sum`, `total` → **Sum** (monotonic, cumulative)
- everything else → **Gauge**

## Emission rules

- Resource attributes: `service.name = <application>`,
  `service.instance.id = <instance>`, `source.address = <sender IP>` (the
  UDP packet's source IP as seen by the receiver, so it may be a NAT or proxy
  address).
- Scope name: `storjstats`.
- **Gauge** metric name is the stat's base name; the field becomes the
  `field` data-point attribute. All tags become data-point attributes too.
  Multiple gauge fields for the same base name group into one metric.
- **Sum** metric name is `<base>_<field>` (e.g. `upload_success_size_bytes_count`).
  Tags become data-point attributes; no `field` attribute is added.

This split resolves the OTel constraint that one metric name has exactly
one aggregation type, and mirrors the Prometheus `_count` / `_sum`
convention.

## Internal telemetry

The receiver reports the standard collector receiver metrics via
`receiverhelper.ObsReport`, exposed on the collector's own telemetry endpoint
(e.g. `:8888/metrics`) with `receiver="storjstats[/name]"` and
`transport="udp"` labels:

- `otelcol_receiver_accepted_metric_points_total`
- `otelcol_receiver_refused_metric_points_total`
- `otelcol_receiver_failed_metric_points_total` (only with the
  `receiverhelper.newReceiverMetrics` feature gate)

Counts are data points forwarded after include-rule filtering; samples dropped
by the rules or unparseable packets are not counted.

[statreceiver]: https://github.com/storj/statreceiver
