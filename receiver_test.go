package storjstatsreceiver

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/admission/v3/admproto"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

// buildPacket encodes an admproto packet with the given samples.
func buildPacket(t *testing.T, application string, instance []byte, samples map[string]float64) []byte {
	t.Helper()
	w := admproto.NewWriterWith(admproto.Options{})
	out, err := w.Begin(nil, application, instance, 0)
	require.NoError(t, err)
	for k, v := range samples {
		out, err = w.Append(out, k, v)
		require.NoError(t, err)
	}
	return admproto.AddChecksum(out)
}

// freeUDPPort finds a free UDP port by binding to :0 and releasing it.
func freeUDPPort(t *testing.T) string {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	require.NoError(t, err)
	conn, err := net.ListenUDP("udp", addr)
	require.NoError(t, err)
	defer conn.Close()
	return conn.LocalAddr().String()
}

func TestReceiver_EndToEnd(t *testing.T) {
	endpoint := freeUDPPort(t)

	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = endpoint
	cfg.Include = []IncludeRule{
		{Name: "upload_success_size_bytes", Fields: []string{"count", "sum"}},
		{Name: "upload_success_duration_ns", Fields: []string{"ravg", "count"}},
		{Name: "pieces_writer", Tags: map[string]string{"size": "2m"}, Fields: []string{"ravg"}},
	}

	sink := &consumertest.MetricsSink{}
	settings := receivertest.NewNopSettings(typeStr)
	r, err := createMetrics(context.Background(), settings, cfg, sink)
	require.NoError(t, err)

	require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		_ = r.Shutdown(context.Background())
	})

	pkt := buildPacket(t, "storagenode", []byte("node-1"), map[string]float64{
		"upload_success_size_bytes count":  42,
		"upload_success_size_bytes sum":    123456,
		"upload_success_duration_ns ravg":  17,
		"upload_success_duration_ns count": 42,
		"pieces_writer,size=2m ravg":       9,
		"noisy_dropped_metric value":       999, // must NOT appear in sink
	})

	client, err := net.Dial("udp", endpoint)
	require.NoError(t, err)
	defer client.Close()
	_, err = client.Write(pkt)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.DataPointCount() >= 5
	}, 2*time.Second, 20*time.Millisecond, "waiting for consumed metrics")

	// Aggregate metric-name → data-point count across all received batches.
	names := map[string]int{}
	types := map[string]pmetric.MetricType{}
	var seenApp, seenInst string
	for _, md := range sink.AllMetrics() {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rm := md.ResourceMetrics().At(i)
			if v, ok := rm.Resource().Attributes().Get("service.name"); ok {
				seenApp = v.AsString()
			}
			if v, ok := rm.Resource().Attributes().Get("service.instance.id"); ok {
				seenInst = v.AsString()
			}
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				for k := 0; k < sm.Metrics().Len(); k++ {
					m := sm.Metrics().At(k)
					var dpCount int
					switch m.Type() {
					case pmetric.MetricTypeGauge:
						dpCount = m.Gauge().DataPoints().Len()
					case pmetric.MetricTypeSum:
						dpCount = m.Sum().DataPoints().Len()
					}
					names[m.Name()] += dpCount
					types[m.Name()] = m.Type()
				}
			}
		}
	}

	assert.Equal(t, "storagenode", seenApp)
	assert.Equal(t, "node-1", seenInst)

	// Sum-typed metrics get _count / _sum suffix.
	assert.Equal(t, 1, names["upload_success_size_bytes_count"])
	assert.Equal(t, 1, names["upload_success_size_bytes_sum"])
	assert.Equal(t, 1, names["upload_success_duration_ns_count"])
	assert.Equal(t, pmetric.MetricTypeSum, types["upload_success_size_bytes_count"])
	assert.Equal(t, pmetric.MetricTypeSum, types["upload_success_duration_ns_count"])

	// Gauge groups ravg into the bare-name metric.
	assert.Equal(t, 1, names["upload_success_duration_ns"])
	assert.Equal(t, pmetric.MetricTypeGauge, types["upload_success_duration_ns"])

	assert.Equal(t, 1, names["pieces_writer"])
	assert.Equal(t, pmetric.MetricTypeGauge, types["pieces_writer"])

	// Dropped metric absent.
	_, present := names["noisy_dropped_metric"]
	assert.False(t, present, "unmatched metric leaked into sink")
}
