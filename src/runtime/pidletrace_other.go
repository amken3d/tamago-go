// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !tamago

package runtime

// tamagoPidleTrace is the no-op form for every platform that is not the one
// this diagnostic exists for.
//
// It has to exist at all because the call site is in proc.go, which every GOOS
// compiles. Putting the definition beside the platform it serves and the call
// beside the code it observes broke the runtime build for every other target --
// including, awkwardly, the host build that `go tool addr2line` performs.
//
//go:nosplit
func tamagoPidleTrace(pid int32, gcw bool, pc uintptr) {}
