// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.
//
// UDP listener adapted from storj.io/statreceiver/udp.go.

package storjstatsreceiver

import (
	"errors"
	"net"
	"sync"
	"time"
)

// udpSource is a UDP packet source that reads one datagram at a time.
type udpSource struct {
	address string
	bufSize int

	mu     sync.Mutex
	conn   *net.UDPConn
	buf    []byte
	closed bool
}

// newUDPSource creates a udpSource listening on address. bufSize is the
// per-packet read buffer in bytes.
func newUDPSource(address string, bufSize int) *udpSource {
	return &udpSource{address: address, bufSize: bufSize}
}

// Start binds the UDP socket. Must be called before Next.
func (s *udpSource) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("udp source closed")
	}
	addr, err := net.ResolveUDPAddr("udp", s.address)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	s.conn = conn
	s.buf = make([]byte, s.bufSize)
	return nil
}

// Addr returns the bound local address, or nil before Start.
func (s *udpSource) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}
	return s.conn.LocalAddr()
}

// Next blocks until the next datagram arrives, then returns its bytes and
// receipt time. The returned slice aliases an internal buffer; copy it if you
// need to retain it past the next call.
func (s *udpSource) Next() ([]byte, time.Time, error) {
	s.mu.Lock()
	conn := s.conn
	buf := s.buf
	closed := s.closed
	s.mu.Unlock()

	if closed {
		return nil, time.Time{}, errors.New("udp source closed")
	}
	if conn == nil {
		return nil, time.Time{}, errors.New("udp source not started")
	}

	n, _, err := conn.ReadFrom(buf)
	if err != nil {
		return nil, time.Time{}, err
	}
	return buf[:n], time.Now(), nil
}

// Close stops the source and unblocks any in-flight Next.
func (s *udpSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
