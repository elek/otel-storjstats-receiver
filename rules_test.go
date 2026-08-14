package storjstatsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func compileRules(t *testing.T, rules []IncludeRule) []compiledRule {
	t.Helper()
	compiled, err := compileIncludeRules(rules)
	require.NoError(t, err)
	return compiled
}

func TestCompileIncludeRules_BadRegex(t *testing.T) {
	_, err := compileIncludeRules([]IncludeRule{{Name: "(unclosed"}})
	require.Error(t, err)
}

func TestCompileIncludeRules_BadType(t *testing.T) {
	_, err := compileIncludeRules([]IncludeRule{{Name: "ok", Type: "histogram"}})
	require.Error(t, err)
}

func TestMatchRules_NameRegexAnchored(t *testing.T) {
	rules := compileRules(t, []IncludeRule{{Name: "hashstore.*"}})

	m, ok := matchRules(rules, statKey{name: "hashstore_foo"})
	require.True(t, ok)
	assert.Equal(t, otelGauge, m.resolvedType)

	_, ok = matchRules(rules, statKey{name: "pre_hashstore_foo"})
	assert.False(t, ok, "regex should be anchored — no substring matches")
}

func TestMatchRules_TagsRequireAllListed(t *testing.T) {
	rules := compileRules(t, []IncludeRule{
		{Name: "pieces_writer", Tags: map[string]string{"size": "2m"}, Fields: []string{"ravg"}},
	})

	// Extra tags are allowed.
	_, ok := matchRules(rules, statKey{
		name:  "pieces_writer",
		tags:  map[string]string{"size": "2m", "op": "write"},
		field: "ravg",
	})
	assert.True(t, ok)

	// Missing required tag → no match.
	_, ok = matchRules(rules, statKey{name: "pieces_writer", tags: map[string]string{}, field: "ravg"})
	assert.False(t, ok)

	// Wrong value → no match.
	_, ok = matchRules(rules, statKey{
		name:  "pieces_writer",
		tags:  map[string]string{"size": "4m"},
		field: "ravg",
	})
	assert.False(t, ok)
}

func TestMatchRules_FieldWhitelist(t *testing.T) {
	rules := compileRules(t, []IncludeRule{
		{Name: "upload_success_size_bytes", Fields: []string{"count", "sum"}},
	})

	_, ok := matchRules(rules, statKey{name: "upload_success_size_bytes", field: "count"})
	assert.True(t, ok)
	_, ok = matchRules(rules, statKey{name: "upload_success_size_bytes", field: "sum"})
	assert.True(t, ok)
	_, ok = matchRules(rules, statKey{name: "upload_success_size_bytes", field: "ravg"})
	assert.False(t, ok)
}

func TestMatchRules_EmptyFieldsAllowsAny(t *testing.T) {
	rules := compileRules(t, []IncludeRule{{Name: "blobs_usage"}})

	_, ok := matchRules(rules, statKey{name: "blobs_usage", field: "anything"})
	assert.True(t, ok)
	_, ok = matchRules(rules, statKey{name: "blobs_usage", field: ""})
	assert.True(t, ok)
}

func TestMatchRules_FirstMatchWins(t *testing.T) {
	rules := compileRules(t, []IncludeRule{
		{Name: "thing", Type: "sum"},
		{Name: "thing", Type: "gauge"},
	})

	m, ok := matchRules(rules, statKey{name: "thing", field: "ravg"})
	require.True(t, ok)
	assert.Equal(t, otelSum, m.resolvedType)
}

func TestMatchRules_HeuristicWhenNoTypeOverride(t *testing.T) {
	rules := compileRules(t, []IncludeRule{{Name: ".*"}})

	cases := []struct {
		field string
		want  otelMetricType
	}{
		{"count", otelSum},
		{"sum", otelSum},
		{"total", otelSum},
		{"ravg", otelGauge},
		{"r99", otelGauge},
		{"min", otelGauge},
		{"max", otelGauge},
		{"value", otelGauge},
		{"active", otelGauge},
		{"length", otelGauge},
		{"", otelGauge},
	}
	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			m, ok := matchRules(rules, statKey{name: "x", field: c.field})
			require.True(t, ok)
			assert.Equal(t, c.want, m.resolvedType)
		})
	}
}

func TestMatchRules_TypeOverrideBeatsHeuristic(t *testing.T) {
	rules := compileRules(t, []IncludeRule{{Name: ".*", Type: "gauge"}})

	// "count" would heuristic to Sum; override forces Gauge.
	m, ok := matchRules(rules, statKey{name: "x", field: "count"})
	require.True(t, ok)
	assert.Equal(t, otelGauge, m.resolvedType)
}
