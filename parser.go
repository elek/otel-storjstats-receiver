// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.
//
// admproto packet decoding adapted from storj.io/statreceiver/parser.go.

package storjstatsreceiver

import (
	"sync"

	"github.com/zeebo/admission/v3/admproto"
)

const parserScratchSize = 10 * 1024

// sample is one decoded (key, value) pair with the packet's application and
// instance.
type sample struct {
	application string
	instance    string
	key         []byte
	value       float64
}

// packetParser decodes admproto packets into samples. Not safe for concurrent
// use from multiple goroutines.
type packetParser struct {
	scratch sync.Pool
}

func newPacketParser() *packetParser {
	return &packetParser{
		scratch: sync.Pool{
			New: func() interface{} {
				b := make([]byte, parserScratchSize)
				return &b
			},
		},
	}
}

// Parse decodes one datagram. samples are only valid until the next call to
// Parse — copy the key bytes if you need to retain them.
func (p *packetParser) Parse(data []byte, out []sample) ([]sample, error) {
	data, err := admproto.CheckChecksum(data)
	if err != nil {
		return out, err
	}

	scratch := p.scratch.Get().(*[]byte)
	defer p.scratch.Put(scratch)

	r := admproto.NewReaderWith(*scratch)
	data, appb, instb, numHeaders, err := r.Begin(data)
	if err != nil {
		return out, err
	}

	for i := 0; i < numHeaders; i++ {
		data, _, _, err = r.NextHeader(data)
		if err != nil {
			return out, err
		}
	}

	app, inst := string(appb), string(instb)
	for len(data) > 0 {
		var key []byte
		var value float64
		data, key, value, err = r.Next(data)
		if err != nil {
			return out, err
		}
		// key aliases the reader's scratch buffer; copy so it survives.
		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)
		out = append(out, sample{
			application: app,
			instance:    inst,
			key:         keyCopy,
			value:       value,
		})
	}
	return out, nil
}
