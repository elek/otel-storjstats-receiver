package storjstatsreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

var typeStr = component.MustNewType("storjstats")

// NewFactory returns a receiver factory for the storjstats receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetrics, component.StabilityLevelAlpha),
	)
}

func createMetrics(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	next consumer.Metrics,
) (receiver.Metrics, error) {
	c := cfg.(*Config)
	rules, err := compileIncludeRules(c.Include)
	if err != nil {
		return nil, err
	}
	return &storjstatsReceiver{
		cfg:      c,
		rules:    rules,
		logger:   settings.Logger,
		consumer: next,
	}, nil
}
