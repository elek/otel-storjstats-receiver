package storjstatsreceiver

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

// Config is the storjstats receiver configuration.
type Config struct {
	Endpoint       string        `mapstructure:"endpoint"`
	ReadBufferSize int           `mapstructure:"read_buffer_size"`
	Include        []IncludeRule `mapstructure:"include"`
}

var _ component.Config = (*Config)(nil)

const defaultReadBufferSize = 10 * 1024

func createDefaultConfig() component.Config {
	return &Config{
		ReadBufferSize: defaultReadBufferSize,
	}
}

// Validate is called by the collector after unmarshal.
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("storjstats: endpoint is required")
	}
	if len(c.Include) == 0 {
		return errors.New("storjstats: at least one include rule is required")
	}
	if _, err := compileIncludeRules(c.Include); err != nil {
		return err
	}
	return nil
}
