// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package signal

import (
	"os"
	"syscall"
	_ "unsafe"
)

var (
	loopG uintptr
	sig   syscall.Signal
)

// Defined by the runtime package.
func getgp() uintptr

// loop is defined per architecture: see signal_tamago_polled.go (arm) and
// signal_tamago_waked.go (everything else).

func init() {
	watchSignalLoop = loop
}

const numSig = 256

func signum(sig os.Signal) int {
	switch sig := sig.(type) {
	case syscall.Signal:
		i := int(sig)
		if i < 0 || i >= numSig {
			return -1
		}
		return i
	default:
		return -1
	}
}

func enableSignal(sig int)  {}
func disableSignal(sig int) {}
func ignoreSignal(sig int)  {}

func signalIgnored(sig int) bool {
	return false
}

//go:linkname waitUntilIdle os/signal.signalWaitUntilIdle
func waitUntilIdle() {
	for !Waiting() {
	}
}

// Relay sends a signal to the [Notify] channel.
//
// To make it suitable for invocation in bare metal interrupt/exception
// handlers, the function is implemented in assembly avoiding allocation and
// runtime use.
//
// It records the signal and sets a pending flag, and does nothing else. In
// particular it does NOT wake loop directly.
//
// It used to, via runtime.WakeG, which located loop's cached time.Sleep timer in
// the runtime timer heap and patched it to fire immediately. That is a data race
// that cannot be fixed in place. runtime.findTimer loads the heap slice's length
// and data pointer as two separate words and then dereferences entries, while
// the scheduler mutates that same heap under ts.lock -- including appends that
// reallocate it. An interrupt landing inside such a mutation reads a torn slice
// header and walks off a stale or zeroed base, and WakeG's subsequent writes
// land on an entry that has already moved. Observed on a Cortex-A53 as a data
// abort reading a low address ((len-1)*sizeof(timerWhen) off a nil base) and as
// "traceback did not unwind completely" on the signal goroutine, whose timer
// state had been corrupted. Taking ts.lock from interrupt context is not an
// option: the interrupted code may already hold it. SMP makes it strictly worse,
// since another core can be mid-mutation rather than merely interleaved.
//
//go:nosplit
func Relay(sig syscall.Signal)

// Waiting returns whether package signal is blocked waiting an incoming signal
// to [Notify].
//
//go:nosplit
func Waiting() bool
