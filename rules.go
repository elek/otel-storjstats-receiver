package storjstatsreceiver

import (
	"fmt"
	"regexp"
)

type otelMetricType int

const (
	otelGauge otelMetricType = iota
	otelSum
)

// IncludeRule is one entry in the receiver's whitelist. Exported because it
// is unmarshalled from the collector config.
type IncludeRule struct {
	// Name is a Go regex matched against the metric's base name (the part
	// before the first comma in the stat key). It is auto-anchored (^…$).
	Name string `mapstructure:"name"`
	// Tags lists tag key=value pairs that all must be present in the sample.
	// Extra tags on the sample are ignored.
	Tags map[string]string `mapstructure:"tags"`
	// Fields is a whitelist of allowed field suffixes. Empty allows any.
	Fields []string `mapstructure:"fields"`
	// Type overrides the metric-type heuristic. "gauge", "sum", or "".
	Type string `mapstructure:"type"`
}

type compiledRule struct {
	name       *regexp.Regexp
	tags       map[string]string
	fields     map[string]struct{} // nil = any
	typeOverr  otelMetricType
	hasTypeOvr bool
}

type ruleMatch struct {
	resolvedType otelMetricType
}

func compileIncludeRules(rules []IncludeRule) ([]compiledRule, error) {
	compiled := make([]compiledRule, 0, len(rules))
	for i, r := range rules {
		re, err := regexp.Compile("^(?:" + r.Name + ")$")
		if err != nil {
			return nil, fmt.Errorf("include[%d]: bad regex %q: %w", i, r.Name, err)
		}

		cr := compiledRule{
			name: re,
			tags: r.Tags,
		}
		if len(r.Fields) > 0 {
			cr.fields = make(map[string]struct{}, len(r.Fields))
			for _, f := range r.Fields {
				cr.fields[f] = struct{}{}
			}
		}

		switch r.Type {
		case "":
			// heuristic
		case "gauge":
			cr.typeOverr, cr.hasTypeOvr = otelGauge, true
		case "sum":
			cr.typeOverr, cr.hasTypeOvr = otelSum, true
		default:
			return nil, fmt.Errorf("include[%d]: unknown type %q (want gauge|sum)", i, r.Type)
		}

		compiled = append(compiled, cr)
	}
	return compiled, nil
}

// matchRules returns the first matching rule's resolved OTel type, or false
// if no rule matches.
func matchRules(rules []compiledRule, k statKey) (ruleMatch, bool) {
	for _, r := range rules {
		if !r.name.MatchString(k.name) {
			continue
		}
		if !tagsSubsetMatch(r.tags, k.tags) {
			continue
		}
		if r.fields != nil {
			if _, ok := r.fields[k.field]; !ok {
				continue
			}
		}
		return ruleMatch{resolvedType: r.resolveType(k.field)}, true
	}
	return ruleMatch{}, false
}

func (r compiledRule) resolveType(field string) otelMetricType {
	if r.hasTypeOvr {
		return r.typeOverr
	}
	return heuristicType(field)
}

// heuristicType picks Sum for cumulative counter fields, Gauge otherwise.
func heuristicType(field string) otelMetricType {
	switch field {
	case "count", "sum", "total":
		return otelSum
	default:
		return otelGauge
	}
}

func tagsSubsetMatch(required, actual map[string]string) bool {
	for k, v := range required {
		if actual[k] != v {
			return false
		}
	}
	return true
}
