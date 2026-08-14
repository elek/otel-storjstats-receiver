package storjstatsreceiver

import (
	"context"
	"errors"
	"net"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type storjstatsReceiver struct {
	cfg      *Config
	rules    []compiledRule
	logger   *zap.Logger
	consumer consumer.Metrics

	source *udpSource
	parser *packetParser

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (r *storjstatsReceiver) Start(_ context.Context, _ component.Host) error {
	bufSize := r.cfg.ReadBufferSize
	if bufSize <= 0 {
		bufSize = defaultReadBufferSize
	}
	r.source = newUDPSource(r.cfg.Endpoint, bufSize)
	if err := r.source.Start(); err != nil {
		return err
	}
	r.parser = newPacketParser()

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	r.wg.Add(1)
	go r.readLoop(ctx)

	r.logger.Info("storjstats receiver started", zap.String("endpoint", r.cfg.Endpoint))
	return nil
}

func (r *storjstatsReceiver) Shutdown(_ context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	if r.source != nil {
		_ = r.source.Close()
	}
	r.wg.Wait()
	return nil
}

func (r *storjstatsReceiver) readLoop(ctx context.Context) {
	defer r.wg.Done()

	var samples []sample
	for {
		if ctx.Err() != nil {
			return
		}
		data, ts, err := r.source.Next()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if isNetClosedErr(err) {
				return
			}
			r.logger.Debug("udp read failed", zap.Error(err))
			continue
		}

		samples = samples[:0]
		samples, err = r.parser.Parse(data, samples)
		if err != nil {
			r.logger.Debug("packet parse failed", zap.Error(err))
			continue
		}
		if len(samples) == 0 {
			continue
		}

		md := convertToMetrics(samples, r.rules, ts)
		if md.ResourceMetrics().Len() == 0 {
			continue
		}
		if err := r.consumer.ConsumeMetrics(ctx, md); err != nil {
			r.logger.Warn("consume metrics failed", zap.Error(err))
		}
	}
}

func isNetClosedErr(err error) bool {
	return errors.Is(err, net.ErrClosed)
}
