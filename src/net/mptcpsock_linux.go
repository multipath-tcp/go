// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import (
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
	case errors.Is(err, syscall.EPROTONOSUPPORT): // MPTCP not supported
	case errors.Is(err, syscall.EINVAL): // MPTCP not supported
	case err == nil:
		poll.CloseFunc(s)
		fallthrough
	default:
		mptcpAvailable = true
	}
}
