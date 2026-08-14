package storjstatsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/admission/v3/admproto"
)

func TestPacketParser_RoundTrip(t *testing.T) {
	w := admproto.NewWriterWith(admproto.Options{})
	out, err := w.Begin(nil, "app", []byte("inst"), 0)
	require.NoError(t, err)
	out, err = w.Append(out, "foo bar", 1.5)
	require.NoError(t, err)
	out, err = w.Append(out, "baz qux", 42)
	require.NoError(t, err)
	pkt := admproto.AddChecksum(out)

	p := newPacketParser()
	samples, err := p.Parse(pkt, nil)
	require.NoError(t, err)
	require.Len(t, samples, 2)
	assert.Equal(t, "app", samples[0].application)
	assert.Equal(t, "inst", samples[0].instance)
	assert.Equal(t, "foo bar", string(samples[0].key))
	assert.InDelta(t, 1.5, samples[0].value, 0.01)
	assert.Equal(t, "baz qux", string(samples[1].key))
	assert.InDelta(t, 42.0, samples[1].value, 0.5)
}
