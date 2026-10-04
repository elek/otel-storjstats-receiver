package storjstatsreceiver

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const scopeName = "storjstats"

var processStartTime = time.Now()

// convertToMetrics filters samples through the compiled rules and builds a
// pmetric.Metrics ready to hand to a consumer.
//
// Grouping (per resource, i.e. per unique (application, instance)):
//   - Gauge → one metric named <base>, one data point per (tags, field). Field
//     goes into the "field" attribute.
//   - Sum   → one metric named <base>_<field>, one data point per tag set.
//     No "field" attribute.
//
// sourceIP is the sender's address, recorded as the "source.address" resource
// attribute; it is omitted when empty.
//
// An empty output has zero ResourceMetrics.
func convertToMetrics(samples []sample, rules []compiledRule, sourceIP string, ts time.Time) pmetric.Metrics {
	md := pmetric.NewMetrics()
	if len(samples) == 0 {
		return md
	}

	// Bucket by resource identity first — one packet always has one (app,
	// instance) but the batch API is flexible so we keep the grouping general.
	type resourceKey struct{ app, inst string }
	type metricKey struct {
		otelName string
		otelType otelMetricType
	}
	type bucket struct {
		metrics map[metricKey]pmetric.Metric
		scope   pmetric.ScopeMetrics
	}
	resources := map[resourceKey]*bucket{}

	timestamp := pcommon.NewTimestampFromTime(ts)
	startTimestamp := pcommon.NewTimestampFromTime(processStartTime)

	for _, s := range samples {
		key := parseStatKey(s.key)
		match, ok := matchRules(rules, key)
		if !ok {
			continue
		}

		rk := resourceKey{app: s.application, inst: s.instance}
		b, exists := resources[rk]
		if !exists {
			rm := md.ResourceMetrics().AppendEmpty()
			rm.Resource().Attributes().PutStr("service.name", s.application)
			rm.Resource().Attributes().PutStr("service.instance.id", s.instance)
			if sourceIP != "" {
				rm.Resource().Attributes().PutStr("source.address", sourceIP)
			}
			sm := rm.ScopeMetrics().AppendEmpty()
			sm.Scope().SetName(scopeName)
			b = &bucket{
				metrics: map[metricKey]pmetric.Metric{},
				scope:   sm,
			}
			resources[rk] = b
		}

		otelName := key.name
		if match.resolvedType == otelSum && key.field != "" {
			otelName = key.name + "_" + key.field
		}
		mk := metricKey{otelName: otelName, otelType: match.resolvedType}

		m, ok := b.metrics[mk]
		if !ok {
			m = b.scope.Metrics().AppendEmpty()
			m.SetName(otelName)
			switch match.resolvedType {
			case otelGauge:
				m.SetEmptyGauge()
			case otelSum:
				sum := m.SetEmptySum()
				sum.SetIsMonotonic(true)
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			}
			b.metrics[mk] = m
		}

		switch match.resolvedType {
		case otelGauge:
			dp := m.Gauge().DataPoints().AppendEmpty()
			dp.SetTimestamp(timestamp)
			dp.SetDoubleValue(s.value)
			putStrAttrs(dp.Attributes(), key.tags)
			dp.Attributes().PutStr("field", key.field)
		case otelSum:
			dp := m.Sum().DataPoints().AppendEmpty()
			dp.SetStartTimestamp(startTimestamp)
			dp.SetTimestamp(timestamp)
			dp.SetDoubleValue(s.value)
			putStrAttrs(dp.Attributes(), key.tags)
		}
	}

	return md
}

func putStrAttrs(attrs pcommon.Map, tags map[string]string) {
	for k, v := range tags {
		attrs.PutStr(k, v)
	}
}
