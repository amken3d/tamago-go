// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build tamago && !arm

package signal

import (
	"time"
)

// This is the original signal loop, kept unchanged for the architectures whose
// [Relay] still wakes it through runtime.WakeG.
//
// That mechanism is racy on every architecture -- runtime.findTimer reads the
// timer heap's length and data pointer as separate words and then dereferences
// entries, while the scheduler mutates that heap under ts.lock -- and it is what
// signal_tamago_polled.go replaces on arm, where the failure was reproduced and
// the fix verified on hardware. It is left in place here rather than ported
// blind: the port is per-architecture assembly that cannot be built or run in
// that environment, and a wrong memory barrier would trade a rare crash for a
// permanently missed interrupt, which is the worse failure.

func loop() {
	loopG = getgp()

	for {
		// Sleep indefinitely until woken up by [Relay] through
		// runtime.wakeg.
		time.Sleep(1<<63 - 1) // math.MaxInt64
		process(sig)
		sig = -1
	}
}
