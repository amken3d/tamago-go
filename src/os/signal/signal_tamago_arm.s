// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

// func Relay(sig syscall.Signal)
//
// Runs in interrupt context. It records the signal and raises the pending flag
// that loop polls -- and nothing more. See the Relay comment in signal_tamago.go
// for why it must not reach into the runtime timer heap to wake loop directly.
TEXT ·Relay(SB),NOSPLIT|NOFRAME,$0-4
	MOVW	sig+0(FP), R0
	MOVW	R0, ·sig(SB)

	// Publish sig before pending, so loop cannot observe the flag and then read
	// a stale signal. Matters on SMP, where loop may be polling on another core.
	WORD	$0xf57ff05f	// dmb sy

	MOVW	$1, R0
	MOVW	R0, ·pending(SB)

	WORD	$0xf57ff05f	// dmb sy
	RET

// func Waiting() bool
TEXT ·Waiting(SB),NOSPLIT|NOFRAME,$0-1
	MOVW	·waiting(SB), R0
	MOVB	R0, ret+0(FP)
	RET
