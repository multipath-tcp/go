// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import (
	"context"
	"syscall"
	"testing"
)

func newLocalListenerMPTCP(t *testing.T) Listener {
	l := &ListenConfig{UseMultipathTCP: true}

	ln, err := l.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	return ln
}

func accept(t *testing.T, ln Listener) {
	c, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	tcp, ok := c.(*TCPConn)
	if !ok {
		t.Fatal("struct is not a TCPConn")
	}

	mptcp, err := tcp.MultipathTCP()
	if err != nil {
		t.Fatal(err)
	}

	addr := ln.Addr().String()
	t.Logf("incoming connection to %s with mptcp: %t", addr, mptcp)

	if !mptcp {
		t.Error("incoming connection is not with MPTCP")
	}
}

func dialerMPTCP(t *testing.T, addr string) {
	d := &Dialer{UseMultipathTCP: true}

	c, err := d.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	tcp, ok := c.(*TCPConn)
	if !ok {
		t.Fatal("struct is not a TCPConn")
	}

	mptcp, err := tcp.MultipathTCP()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("outgoing connection from %s with mptcp: %t", addr, mptcp)

	if !mptcp {
		t.Error("incoming connection is not with MPTCP")
	}
}

func canCreateMPTCPSocket() bool {
	// We want to know if we can create an MPTCP socket, not just if it is
	// available (mptcpAvailable()): it could be blocked by the admin
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, _IPPROTO_MPTCP)
	if err != nil {
		return false
	}

	syscall.Close(fd)
	return true
}

func TestMultiPathTCP(t *testing.T) {
	if !canCreateMPTCPSocket() {
		t.Skip("Cannot create MPTCP sockets")
	}

	ln := newLocalListenerMPTCP(t)
	defer ln.Close()
	go func() {
		accept(t, ln)
	}()

	dialerMPTCP(t, ln.Addr().String())
}
