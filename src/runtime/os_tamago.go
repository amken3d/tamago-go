// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build tamago

package runtime

import (
	"internal/abi"
	"internal/runtime/atomic"
	"internal/runtime/math"
	"runtime/goos"
	"unsafe"
)

type mOS struct {
	waitsemacount uint32
}

func hwinit0() {
	goos.Hwinit0()
}

func hwinit1() {
	goos.Hwinit1()
}

func nanotime1() int64 {
	return goos.Nanotime()
}

// GetRandomData generates len(r) random bytes from the random source provided
// externally by the linked application.
func GetRandomData(r []byte) {
	goos.GetRandomData(r)
}

// GetG returns the pointer to the current G and its P.
//
// Deprecated: [os/signal.Notify] and [os/signal.Relay] remove [WakeG] and
// [GetG] need.
func GetG() (gp uint, pp uint)

// WakeG modifies a goroutine cached timer for time.Sleep (g.timer) to fire as
// soon as possible.
//
// The function is meant to be invoked within Go assembly and its arguments
// must be passed through registers rather than on the frame pointer, see
// definition in sys_tamago_$GOARCH.s for details.
//
// Deprecated: [os/signal.Notify] and [os/signal.Relay] remove [WakeG] and
// [GetG] need.
func WakeG()

// Wake modifies a goroutine cached timer for time.Sleep (g.timer) to fire as
// soon as possible, reporting whether the modification is successful.
//
// Deprecated: [os/signal.Notify] and [os/signal.Relay] remove [Wake] and
// [GetG] need.
func Wake(gp uint) bool

//go:linkname getgp os/signal.getgp
func getgp() (gp uintptr) {
	return uintptr(unsafe.Pointer(getg()))
}

// stubs for unused/unimplemented functionality
type sigset struct{}
type gsignalStack struct{}

func goenvs()                        {}
func sigsave(p *sigset)              {}
func msigrestore(sigmask sigset)     {}
func clearSignalHandlers()           {}
func sigblock(exiting bool)          {}
func unminit()                       {}
func mdestroy(mp *m)                 {}
func setProcessCPUProfiler(hz int32) {}
func setThreadCPUProfiler(hz int32)  {}
func initsig(preinit bool)           {}
func osyield()                       {}
func osyield_no_g()                  {}

const stacksize = 8192 * 1024 // 8192KB

// May run with m.p==nil, so write barriers are not allowed.
//
//go:nowritebarrier
func newosproc(mp *m) {
	if goos.Task == nil {
		throw("newosproc: not implemented")
	}

	stack := sysAlloc(stacksize, &memstats.stacks_sys, "HW thread stack")
	if stack == nil {
		writeErrStr(failallocatestack)
		exit(1)
	}

	goos.Task(unsafe.Pointer(uintptr(stack)+stacksize), unsafe.Pointer(mp), unsafe.Pointer(mp.g0), unsafe.Pointer(abi.FuncPCABI0(mstart)))
}

// Called to initialize a new m (including the bootstrap m).
// Called on the parent thread (main thread in case of bootstrap), can allocate memory.
func mpreinit(mp *m) {
	mp.gsignal = malg(32 * 1024)
	mp.gsignal.m = mp
}

func getCPUCount() int32 {
	return numCPUStartup
}

func osinit() {
	physPageSize = 4096

	// SMP-capable platforms set goos.NumCPU (before osinit, in Hwinit0)
	// and provide goos.Task to start the additional processors; the
	// default is uniprocessor operation.
	numCPUStartup = goos.NumCPU

	if numCPUStartup < 1 {
		numCPUStartup = 1
	}

	if goos.Bloc != 0 {
		bloc = goos.Bloc
		blocMax = bloc
	} else {
		initBloc()
	}
}

func readRandom(r []byte) int {
	goos.InitRNG()
	goos.GetRandomData(r)
	return len(r)
}

func signame(sig uint32) string {
	return ""
}

//go:linkname os_sigpipe os.sigpipe
func os_sigpipe() {
	throw("too many writes on closed pipe")
}

//go:nosplit
func crash() {
	*(*int32)(nil) = 0
}

//go:linkname syscall
func syscall(number, a1, a2, a3 uintptr) (r1, r2, err uintptr) {
	switch number {
	// SYS_WRITE
	case 1:
		r1 := write(a1, unsafe.Pointer(a2), int32(a3))
		return uintptr(r1), 0, 0
	default:
		throw("unexpected syscall")
	}

	return
}

//go:nosplit
func write1(fd uintptr, buf unsafe.Pointer, count int32) int32 {
	if fd != 1 && fd != 2 {
		throw("unexpected fd, only stdout/stderr are supported")
	}

	c := uintptr(count)

	for i := uintptr(0); i < c; i++ {
		p := (*byte)(unsafe.Pointer(uintptr(buf) + i))
		goos.Printk(*p)
	}

	return int32(c)
}

//go:linkname syscall_now syscall.now
func syscall_now() (sec int64, nsec int32) {
	sec, nsec, _ = time_now()
	return
}

//go:nosplit
func walltime() (sec int64, nsec int32) {
	nano := nanotime()
	sec = nano / 1000000000
	nsec = int32(nano % 1000000000)
	return
}

//go:nosplit
func usleep(us uint32) {
	wake := nanotime() + int64(us)*1000
	for nanotime() < wake {
	}
}

//go:nosplit
func usleep_no_g(usec uint32) {
	usleep(usec)
}

func exit(code int32) {
	if goos.Exit != nil {
		goos.Exit(code)
	}

	print("exit with code ", code, " halting\n")

	for {
		// hang forever
	}
}

func exitThread(wait *atomic.Uint32) {
	return
}

//go:nosplit
func semacreate(mp *m) {
}

//go:nosplit
func semasleep(ns int64) int {
	var deadline int64
	var v uint32

	gp := getg()
	addr := &gp.m.waitsemacount

	if v = atomic.Load(addr); v > 0 && atomic.Cas(addr, v, v-1) {
		return 0
	}

	if ns >= 0 {
		deadline = nanotime() + ns
	} else {
		deadline = math.MaxInt64
	}

	for {
		if v = atomic.Load(addr); v > 0 {
			if atomic.Cas(addr, v, v-1) {
				return 0
			}
			continue
		}
		if ns >= 0 {
			if deadline-nanotime() <= 0 {
				return -1
			}
		}
		if goos.Idle != nil {
			goos.Idle(deadline)
		}
	}
}

//go:nosplit
func semawakeup(mp *m) {
	atomic.Xadd(&mp.waitsemacount, 1)

	if goos.Wake != nil {
		goos.Wake(mp.procid)
	}
}

const preemptMSupported = true

// preemptM requests asynchronous preemption of mp. The pending flag is set
// and the platform delivers an inter-processor interrupt to the processor
// mp runs on (goos.PreemptM); that processor's interrupt handler brings the
// interrupted goroutine to a safe point -- at minimum by the cooperative
// poison (tamagoPreempt), and at async-safe points by the trap-frame
// redirect (tamagoSigPreempt) when the platform has armed it. Without a
// platform hook the request is dropped, which is the pre-SMP behaviour:
// single-processor configurations preempt via the periodic tick alone.
func preemptM(mp *m) {
	if goos.PreemptM == nil {
		return
	}
	if mp.signalPending.CompareAndSwap(0, 1) {
		goos.PreemptM(mp.procid)
	}
}

// tamagoTrapFrame is the interrupted context as the platform IRQ handler
// lays it out (see the arm irqHandler): banked SP and LR first, then SPSR,
// the general registers, and the adjusted return PC last. The PC slot is
// the handler's pushed R14, so a rewritten pc rides its ordinary return
// path; sp and lr are restored to the banked registers separately.
type tamagoTrapFrame struct {
	sp, lr, spsr uint32
	r            [13]uint32
	pc           uint32
}

// tamagoPreemptCheck decides, from the platform IRQ handler, whether the
// interrupted context should be redirected through asyncPreempt, and hands
// back the signal g (and its stack top) to run the redirect on. It runs on
// the exception stack with the interrupted g still in the g register, so
// like tamagoPreempt it must stay nosplit and treat that g as untrusted.
//
// A zero return means "leave the frame alone": the async tier is not armed
// (goos.AsyncPreempt), no request is pending, or the moment is wrong (no g,
// scheduler or signal g, mid-switch). A wrong moment still consumes the
// request: the cooperative poison is already planted by tamagoPreempt, and
// leaving the flag set would make preemptM drop every later request for
// this m. doSigPreempt acknowledges every delivery for the same reason.
//
//go:linkname tamagoPreemptCheck
//go:nosplit
func tamagoPreemptCheck() (gsig uintptr, sp uintptr) {
	if !goos.AsyncPreempt {
		return 0, 0
	}
	gp := getg()
	if gp == nil {
		return 0, 0
	}
	mp := gp.m
	if mp == nil || mp.gsignal == nil {
		return 0, 0
	}
	if mp.signalPending.Load() == 0 {
		return 0, 0
	}
	if gp == mp.g0 || gp == mp.gsignal || mp.curg != gp {
		mp.preemptGen.Add(1)
		mp.signalPending.Store(0)
		return 0, 0
	}
	sg := mp.gsignal
	return uintptr(unsafe.Pointer(sg)), sg.stack.hi
}

// tamagoSigPreempt is doSigPreempt for the bare-metal trap frame. The
// platform IRQ handler calls it on gp.m's signal stack (g register and SP
// switched, the same move a Unix signal makes) after tamagoPreemptCheck
// accepted the moment. If the interrupted PC is an async-safe point the
// frame is rewritten to enter asyncPreempt -- the ARM pushCall: the old LR
// goes to the interrupted stack, LR becomes the resume PC, PC becomes
// asyncPreempt -- and the request is acknowledged either way.
//
//go:linkname tamagoSigPreempt
func tamagoSigPreempt(frame *tamagoTrapFrame, gp *g) {
	if wantAsyncPreempt(gp) {
		if ok, newpc := isAsyncSafePoint(gp, uintptr(frame.pc), uintptr(frame.sp), uintptr(frame.lr)); ok {
			sp := frame.sp - 4
			*(*uint32)(unsafe.Pointer(uintptr(sp))) = frame.lr
			frame.sp = sp
			frame.lr = uint32(newpc)
			frame.pc = uint32(abi.FuncPCABI0(asyncPreempt))
		}
	}
	gp.m.preemptGen.Add(1)
	gp.m.signalPending.Store(0)
}

// tamagoPreempt requests cooperative preemption of the goroutine that was
// running when an interrupt was taken. It is called from the platform IRQ
// handler (see arm irqHandler), where the g register still holds the
// interrupted goroutine, so getg() returns it.
//
// Bare-metal tamago has no async preemption (preemptMSupported == false) and no
// OS to time-slice threads, so sysmon cannot run to call preemptone while a
// goroutine monopolizes the single core. Poisoning stackguard0 here folds a
// preemption request into the interrupted goroutine's next stack-growth check
// (every function prologue), so it yields at its next call. Combined with a
// periodic timer interrupt on the platform side, this gives the runtime the
// time-slicing it would otherwise get from an OS.
//
//go:linkname tamagoPreempt
//go:nosplit
func tamagoPreempt() {
	gp := getg()
	if gp == nil {
		return
	}

	mp := gp.m
	if mp == nil {
		return
	}

	// never poison the scheduler (g0) or signal (gsignal) stacks
	if gp == mp.g0 || gp == mp.gsignal {
		return
	}

	// The g register is read at an arbitrary interrupt point, so treat it as
	// untrusted: a stale or half-switched value would otherwise pass the nil
	// check and this function would write two words through it, 100 times a
	// second. mp.curg is the m's own record of which g is running, so requiring
	// agreement rejects any g pointer the scheduler does not recognise.
	if mp.curg != gp {
		return
	}

	// Only a running g may be marked for preemption. Touching one mid-transition
	// (_Gcopystack in particular, where newstack is rewriting stackguard0) races
	// the runtime's own bookkeeping.
	if readgstatus(gp)&^_Gscan != _Grunning {
		return
	}

	gp.stackguard0 = stackPreempt
	gp.preempt = true
}

func minit() {
	if goos.ProcID == nil {
		return
	}

	gp := getg()
	gp.m.procid = goos.ProcID()
}

// Stubs so tests can link correctly. These should never be called.
func open(name *byte, mode, perm int32) int32        { panic("not implemented") }
func closefd(fd int32) int32                         { panic("not implemented") }
func read(fd int32, p unsafe.Pointer, n int32) int32 { panic("not implemented") }
