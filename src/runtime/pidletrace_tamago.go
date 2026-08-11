// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build tamago

package runtime

import "runtime/goos"

// pidleTrace is a ring of the last few times a P was put on the idle list.
//
// It exists to answer one question that no amount of reading the code has
// settled: a stop-the-world was observed waiting for exactly one P while that P
// sat in the idle list, with npidle and the list AGREEING (so not corruption)
// and the machine's cache coherency independently proven (so not a stale read).
// Both of those eliminations came from measurements; this one records the
// transition itself.
//
// gcw is sched.gcwaiting AT THE MOMENT of the insertion, read under sched.lock,
// exactly as findRunnable would have read it. If any entry shows a P being
// parked with gcw true, the ordering guarantee is broken and the caller PC says
// by whom. If none does, the P was parked legitimately before the stop began,
// and the bug is that stop-the-world did not take it.
var pidleTrace struct {
	n   uint32
	buf [8]struct {
		pid  int32
		gcw  bool
		pc   uintptr
		tick uint32
	}
}

//go:nosplit
func tamagoPidleTrace(pid int32, gcw bool, pc uintptr) {
	i := pidleTrace.n % uint32(len(pidleTrace.buf))
	pidleTrace.n++

	e := &pidleTrace.buf[i]
	e.pid, e.gcw, e.pc = pid, gcw, pc

	if goos.RawTicks != nil {
		e.tick = goos.RawTicks()
	}
}

//go:nosplit
func dmPidleTrace() {
	dmPuts("[deadman] pidleput ring (newest last); gcw=1 means parked DURING a stop\n")

	n := pidleTrace.n
	if n > uint32(len(pidleTrace.buf)) {
		n = uint32(len(pidleTrace.buf))
	}

	for k := uint32(0); k < n; k++ {
		i := (pidleTrace.n - n + k) % uint32(len(pidleTrace.buf))
		e := &pidleTrace.buf[i]

		dmPuts("[deadman]   P")
		dmHex(uint64(uint32(e.pid)))
		dmPuts(" gcw ")
		if e.gcw {
			dmPuts("1")
		} else {
			dmPuts("0")
		}
		dmPuts(" pc ")
		dmHex(uint64(e.pc))
		dmPuts(" tick ")
		dmHex(uint64(e.tick))
		dmPuts("\n")
	}
}

