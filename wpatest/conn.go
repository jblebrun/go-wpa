package wpatest

import (
	"errors"
	"net"
)

// testConn does its best to emulate what using unix datagrams is like
// it's very simple, and only supports a basic 1:1 communication pattern.
// which is all we need here.
type TestConn struct {
	inmsgs  chan []byte
	outmsgs chan []byte
}

func NewTestConn() (*TestConn, error) {
	return &TestConn{
		inmsgs:  make(chan []byte, 100),
		outmsgs: make(chan []byte, 100),
	}, nil
}

func NewTestListenConn() (*TestConn, error) {
	return &TestConn{
		inmsgs:  make(chan []byte, 100),
		outmsgs: make(chan []byte, 100),
	}, nil
}

func (tc *TestConn) Read(b []byte) (int, error) {
	msg, ok := <-tc.inmsgs
	if !ok {
		return 0, errors.New("closed")
	}
	n := len(b)
	if len(b) > len(msg) {
		n = len(msg)
	}
	copy(b, msg)
	return n, nil
}

func (tc *TestConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, err := tc.Read(b)
	return n, nil, err
}

func (tc *TestConn) Write(b []byte) (int, error) {
	m := make([]byte, len(b))
	copy(m, b)
	tc.outmsgs <- m
	return len(m), nil
}

func (tc *TestConn) Close() error {
	close(tc.inmsgs)
	close(tc.outmsgs)
	return nil
}

// Dial should return something that can communicate with the given connection,
// so the channels are swapped.
func (tc *TestConn) Dial() (*TestConn, error) {
	return &TestConn{
		inmsgs:  tc.outmsgs,
		outmsgs: tc.inmsgs,
	}, nil
}

// Get returns a connection for sending responses, so for tests,
// we just return this connection
func (tc *TestConn) Get(addr net.Addr) (Conn, error) {
	return tc, nil
}
