// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build tamago && arm

package signal

import (
	"sync/atomic"
	"time"
)

// This is the signal loop for architectures whose [Relay] does nothing but
// record a signal and raise a flag. See the comment on Relay in
// signal_tamago.go for why the alternative -- having the interrupt handler wake
// this goroutine by patching its cached timer inside the runtime timer heap --
// is a data race that cannot be fixed in place.
//
// The other architectures still use that mechanism, in signal_tamago_waked.go.
// Porting them is mechanical (Relay stores instead of branching to WakeG) but is
// deliberately not done blind: it is assembly that cannot be built or run here,
// and a wrong barrier would replace a rare race with a permanently missed
// interrupt.

var (
	// pending is raised by [Relay] and cleared by loop. Together with sig it is
	// the ENTIRE interrupt-context side effect.
	pending uint32

	// waiting reports that loop is parked, and backs [Waiting]. It replaces
	// probing the runtime timer heap for loop's cached timer, which was both
	// racy and meaningless once loop polls.
	waiting uint32
)

// relayPoll is how long loop sleeps between checks for a relayed signal, and so
// bounds interrupt-service latency.
//
// It must stay well under the time an unserviced peripheral takes to lose data:
// a 115200 baud UART with an 8-byte receive FIFO overruns in ~694us, so this
// leaves better than 3x margin. It is deliberately no shorter, because every
// tick is a wakeup that denies an idle governor its deep sleep.
const relayPoll = 200 * time.Microsecond

func loop() {
	loopG = getgp()

	for {
		// Poll rather than sleep indefinitely. Bounded sleeps are what let Relay
		// be a plain memory store; the cost is up to relayPoll of added service
		// latency and a wakeup every relayPoll.
		atomic.StoreUint32(&waiting, 1)
		for atomic.SwapUint32(&pending, 0) == 0 {
			time.Sleep(relayPoll)
		}
		atomic.StoreUint32(&waiting, 0)

		// sig is not cleared afterwards. The flag alone decides whether there is
		// work, so a stale sig can never be reprocessed -- whereas clearing it
		// could race a Relay that has just stored a new signal and overwrite it,
		// turning a delivered interrupt into a lost one.
		process(sig)
	}
}
