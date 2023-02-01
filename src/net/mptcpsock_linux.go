// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import (
	"context"
	"errors"
	"internal/poll"
	"sync"
	"syscall"
)

var (
	mptcpOnce      sync.Once
	mptcpAvailable bool
)

// These constants aren't in the syscall package, which is frozen
const (
	_IPPROTO_MPTCP = 0x106
)

func supportsMultipathTCP() bool {
	mptcpOnce.Do(initMPTCPavailable)
	return mptcpAvailable
}

func initMPTCPavailable() {
	s, err := sysSocket(syscall.AF_INET, syscall.SOCK_STREAM, _IPPROTO_MPTCP)
	switch {
	case errors.Is(err, syscall.EPROTONOSUPPORT):
	case errors.Is(err, syscall.EINVAL):
	case err == nil:
		poll.CloseFunc(s)
		fallthrough
	default:
		mptcpAvailable = true
	}
}

func (sd *sysDialer) dialMPTCP(ctx context.Context, laddr, raddr *TCPAddr) (*TCPConn, error) {
	// Fallback to dialTCP if Multipath TCP isn't supported on this operating system.
	if !supportsMultipathTCP() {
		return sd.dialTCP(ctx, laddr, raddr)
	}

	conn, err := sd.doDialTCPProto(ctx, laddr, raddr, _IPPROTO_MPTCP)
	if err == nil {
		return conn, nil
	}

	// Possible MPTCP specific error: ENOPROTOOPT (sysctl net.mptcp.enabled=0)
	// But just in case MPTCP is blocked differently (SELinux, etc.), just
	// retry with "plain" TCP.
	return sd.dialTCP(ctx, laddr, raddr)
}

func (sl *sysListener) listenMPTCP(ctx context.Context, laddr *TCPAddr) (*TCPListener, error) {
	// Fallback to listenTCP if Multipath TCP isn't supported on this operating system.
	if !supportsMultipathTCP() {
		return sl.listenTCP(ctx, laddr)
	}

	dial, err := sl.listenTCPProto(ctx, laddr, _IPPROTO_MPTCP)
	if err == nil {
		return dial, nil
	}

	// Possible MPTCP specific error: ENOPROTOOPT (sysctl net.mptcp.enabled=0)
	// But just in case MPTCP is blocked differently (SELinux, etc.), just
	// retry with "plain" TCP.
	return sl.listenTCP(ctx, laddr)
}
