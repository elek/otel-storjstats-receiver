package storjstatsreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

var typeStr = component.MustNewType("storjstats")

const (
	transportUDP = "udp"
	formatAdm    = "admproto"
)

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
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              transportUDP,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	return &storjstatsReceiver{
		cfg:      c,
		rules:    rules,
		logger:   settings.Logger,
		consumer: next,
		obsrecv:  obsrecv,
	}, nil
}
