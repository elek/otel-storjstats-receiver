package storjstatsreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// findMetric returns the sole metric with the given name from the first
// ScopeMetrics of the first ResourceMetrics; fails the test if absent or
// duplicated.
func findMetric(t *testing.T, md pmetric.Metrics, name string) pmetric.Metric {
	t.Helper()
	require.Equal(t, 1, md.ResourceMetrics().Len())
	sm := md.ResourceMetrics().At(0).ScopeMetrics()
	require.Equal(t, 1, sm.Len())
	metrics := sm.At(0).Metrics()
	var found []pmetric.Metric
	for i := 0; i < metrics.Len(); i++ {
		if metrics.At(i).Name() == name {
			found = append(found, metrics.At(i))
		}
	}
	require.Len(t, found, 1, "want exactly one metric named %q", name)
	return found[0]
}

func TestConvert_ResourceAttributes(t *testing.T) {
	rules := compileRules(t, []IncludeRule{{Name: "x"}})
	now := time.Unix(1_700_000_000, 0)

	md := convertToMetrics([]sample{
		{application: "storagenode", instance: "node-1", key: []byte("x value"), value: 1},
	}, rules, now)

	rm := md.ResourceMetrics()
	require.Equal(t, 1, rm.Len())
	attrs := rm.At(0).Resource().Attributes()
	svc, _ := attrs.Get("service.name")
	inst, _ := attrs.Get("service.instance.id")
	assert.Equal(t, "storagenode", svc.AsString())
	assert.Equal(t, "node-1", inst.AsString())
}

func TestConvert_GaugeGroupsFieldsUnderOneMetric(t *testing.T) {
	rules := compileRules(t, []IncludeRule{{Name: "upload_success_duration_ns"}})
	now := time.Unix(1_700_000_000, 0)

	md := convertToMetrics([]sample{
		{application: "sn", instance: "i", key: []byte("upload_success_duration_ns ravg"), value: 10},
		{application: "sn", instance: "i", key: []byte("upload_success_duration_ns r50"), value: 20},
		{application: "sn", instance: "i", key: []byte("upload_success_duration_ns r99"), value: 30},
	}, rules, now)

	m := findMetric(t, md, "upload_success_duration_ns")
	require.Equal(t, pmetric.MetricTypeGauge, m.Type())
	dps := m.Gauge().DataPoints()
	require.Equal(t, 3, dps.Len())

	got := map[string]float64{}
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		field, _ := dp.Attributes().Get("field")
		got[field.AsString()] = dp.DoubleValue()
	}
	assert.Equal(t, map[string]float64{"ravg": 10, "r50": 20, "r99": 30}, got)
}

func TestConvert_SumFieldsBecomeSeparateMetrics(t *testing.T) {
	rules := compileRules(t, []IncludeRule{{Name: "upload_success_size_bytes", Fields: []string{"count", "sum"}}})
	now := time.Unix(1_700_000_000, 0)

	md := convertToMetrics([]sample{
		{application: "sn", instance: "i", key: []byte("upload_success_size_bytes count"), value: 100},
		{application: "sn", instance: "i", key: []byte("upload_success_size_bytes sum"), value: 12345},
	}, rules, now)

	countM := findMetric(t, md, "upload_success_size_bytes_count")
	sumM := findMetric(t, md, "upload_success_size_bytes_sum")

	require.Equal(t, pmetric.MetricTypeSum, countM.Type())
	require.Equal(t, pmetric.MetricTypeSum, sumM.Type())

	assert.True(t, countM.Sum().IsMonotonic())
	assert.Equal(t, pmetric.AggregationTemporalityCumulative, countM.Sum().AggregationTemporality())

	// Sum metrics have no "field" attribute.
	require.Equal(t, 1, countM.Sum().DataPoints().Len())
	dp := countM.Sum().DataPoints().At(0)
	_, hasField := dp.Attributes().Get("field")
	assert.False(t, hasField)
	assert.Equal(t, float64(100), dp.DoubleValue())
}

func TestConvert_MixedGaugeAndSumForSameBaseName(t *testing.T) {
	rules := compileRules(t, []IncludeRule{{Name: "upload_success_duration_ns"}})
	now := time.Unix(1_700_000_000, 0)

	md := convertToMetrics([]sample{
		{application: "sn", instance: "i", key: []byte("upload_success_duration_ns ravg"), value: 10},
		{application: "sn", instance: "i", key: []byte("upload_success_duration_ns count"), value: 42},
	}, rules, now)

	// One Gauge named after the base, one Sum with _count suffix.
	gauge := findMetric(t, md, "upload_success_duration_ns")
	count := findMetric(t, md, "upload_success_duration_ns_count")
	assert.Equal(t, pmetric.MetricTypeGauge, gauge.Type())
	assert.Equal(t, pmetric.MetricTypeSum, count.Type())
}

func TestConvert_TagsBecomeAttributes(t *testing.T) {
	rules := compileRules(t, []IncludeRule{{Name: "pieces_writer"}})
	now := time.Unix(1_700_000_000, 0)

	md := convertToMetrics([]sample{
		{application: "sn", instance: "i", key: []byte("pieces_writer,size=2m,op=write ravg"), value: 5},
	}, rules, now)

	m := findMetric(t, md, "pieces_writer")
	dp := m.Gauge().DataPoints().At(0)
	got := map[string]string{}
	for _, key := range []string{"size", "op", "field"} {
		v, ok := dp.Attributes().Get(key)
		require.True(t, ok, "missing attribute %s", key)
		got[key] = v.AsString()
	}
	assert.Equal(t, map[string]string{"size": "2m", "op": "write", "field": "ravg"}, got)
}

func TestConvert_DropsUnmatched(t *testing.T) {
	rules := compileRules(t, []IncludeRule{{Name: "keeper"}})
	now := time.Unix(1_700_000_000, 0)

	md := convertToMetrics([]sample{
		{application: "sn", instance: "i", key: []byte("dropped value"), value: 1},
		{application: "sn", instance: "i", key: []byte("keeper value"), value: 2},
	}, rules, now)

	// Only "keeper" should be present.
	sm := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	names := []string{}
	for i := 0; i < sm.Len(); i++ {
		names = append(names, sm.At(i).Name())
	}
	assert.Equal(t, []string{"keeper"}, names)
}

func TestConvert_EmptySamplesEmptyMetrics(t *testing.T) {
	md := convertToMetrics(nil, nil, time.Now())
	assert.Equal(t, 0, md.ResourceMetrics().Len())
}

func TestConvert_ScopeName(t *testing.T) {
	rules := compileRules(t, []IncludeRule{{Name: "x"}})
	now := time.Unix(1_700_000_000, 0)

	md := convertToMetrics([]sample{
		{application: "sn", instance: "i", key: []byte("x value"), value: 1},
	}, rules, now)

	assert.Equal(t, "storjstats", md.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Name())
}
