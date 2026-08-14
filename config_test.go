package storjstatsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate_OK(t *testing.T) {
	c := &Config{
		Endpoint: "0.0.0.0:9000",
		Include: []IncludeRule{
			{Name: "pieces_writer", Tags: map[string]string{"size": "2m"}, Fields: []string{"ravg"}},
		},
	}
	require.NoError(t, c.Validate())
}

func TestConfig_Validate_EmptyEndpoint(t *testing.T) {
	c := &Config{Include: []IncludeRule{{Name: "x"}}}
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint")
}

func TestConfig_Validate_EmptyInclude(t *testing.T) {
	c := &Config{Endpoint: "0.0.0.0:9000"}
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "include")
}

func TestConfig_Validate_BadRule(t *testing.T) {
	c := &Config{Endpoint: "0.0.0.0:9000", Include: []IncludeRule{{Name: "(unterminated"}}}
	err := c.Validate()
	require.Error(t, err)
}

func TestCreateDefaultConfig(t *testing.T) {
	def := createDefaultConfig().(*Config)
	// Endpoint has no sensible default (would clash if two receivers run),
	// but read_buffer_size should have one.
	assert.Equal(t, 10240, def.ReadBufferSize)
}
