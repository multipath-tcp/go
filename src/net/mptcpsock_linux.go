// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import (
	"context"
	"errors"
	"internal/poll"
	"internal/syscall/unix"
	"log"
	"sync"
	"syscall"
)

var (
	mptcpOnce      sync.Once
	mptcpAvailable bool
	hasSOLMPTCP    bool
)

// These constants aren't in the syscall package, which is frozen
const (
	_IPPROTO_MPTCP = 0x106
	_SOL_MPTCP     = 0x11c
	_MPTCP_INFO    = 0x1
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

	log.Printf("MPTCP: check socket: %t (%v)", mptcpAvailable, err)

	major, minor := unix.KernelVersion()
	// SOL_MPTCP only supported from kernel 5.16
	hasSOLMPTCP = major > 5 || (major == 5 && minor >= 16)
	log.Printf("check version: %d.%d: %t", major, minor, hasSOLMPTCP)
}

func (sd *sysDialer) dialMPTCP(ctx context.Context, laddr, raddr *TCPAddr) (*TCPConn, error) {
	// Fallback to dialTCP if Multipath TCP isn't supported on this operating system.
	if !supportsMultipathTCP() {
		log.Printf("MPTCP not supported")
		return sd.dialTCP(ctx, laddr, raddr)
	}

	conn, err := sd.doDialTCPProto(ctx, laddr, raddr, _IPPROTO_MPTCP)
	if err == nil {
		log.Printf("MPTCP Conn created")
		return conn, nil
	}

	log.Printf("Not possible to create MPTCP Socket: %v", err)

	// Possible MPTCP specific error: ENOPROTOOPT (sysctl net.mptcp.enabled=0)
	// But just in case MPTCP is blocked differently (SELinux, etc.), just
	// retry with "plain" TCP.
	return sd.dialTCP(ctx, laddr, raddr)
}

func (sl *sysListener) listenMPTCP(ctx context.Context, laddr *TCPAddr) (*TCPListener, error) {
	// Fallback to listenTCP if Multipath TCP isn't supported on this operating system.
	if !supportsMultipathTCP() {
		log.Printf("MPTCP not supported")
		return sl.listenTCP(ctx, laddr)
	}

	dial, err := sl.listenTCPProto(ctx, laddr, _IPPROTO_MPTCP)
	if err == nil {
		log.Printf("MPTCP List created")
		return dial, nil
	}

	log.Printf("Not possible to create MPTCP List: %v", err)

	// Possible MPTCP specific error: ENOPROTOOPT (sysctl net.mptcp.enabled=0)
	// But just in case MPTCP is blocked differently (SELinux, etc.), just
	// retry with "plain" TCP.
	return sl.listenTCP(ctx, laddr)
}

// Kernel >= 5.16 with SOL_MPTCP support
func useMultipathTCPNew(fd *netFD) bool {
	_, err := fd.pfd.GetsockoptInt(_SOL_MPTCP, _MPTCP_INFO)

	log.Printf("Has SOL MPTCP: %v", err)

	// Error is not the expected one for fallback? MPTCP is then used
	switch fd.family {
	case syscall.AF_INET:
		return err != syscall.EOPNOTSUPP
	case syscall.AF_INET6:
		return err != syscall.ENOPROTOOPT
	}

	return false
}

// Kernel < 5.16 without SOL_MPTCP support
func useMultipathTCPOld(fd *netFD) bool {
	// Less good: only check if the protocol being used is MPTCP
	proto, _ := fd.pfd.GetsockoptInt(syscall.SOL_SOCKET, syscall.SO_PROTOCOL)

	log.Printf("Doesn't have SOL MPTCP: %d", proto)

	return proto == _IPPROTO_MPTCP
}

// Check if MPTCP is being used
func useMultipathTCP(fd *netFD) bool {
	// Kernel >= 5.16 returns EOPNOTSUPP/ENOPROTOOPT in case of fallback.
	// Older kernels will always return them: not usable
	if hasSOLMPTCP {
		return useMultipathTCPNew(fd)
	}

	return useMultipathTCPOld(fd)
}
