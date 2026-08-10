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

	// Deliver the payload whole when the platform accepts writes at that
	// granularity (see goos.Write): an SMP console serializes whole writes
	// where per-byte delivery would interleave concurrent writers mid-line.
	// The slice header is built by hand (nosplit context), and the buffer
	// pointer is laundered through noescape: the callers' print buffers
	// live on the stack and must not be flagged as escaping through the
	// indirect call -- the hook's contract is that it does not retain p.
	if goos.Write != nil && count > 0 {
		var b []byte
		s := (*slice)(unsafe.Pointer(&b))
		s.array = noescape(buf)
		s.len = int(count)
		s.cap = int(count)
		goos.Write(b)
		return int32(c)
	}

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
//
// Nosplit because tamagoTimeSlice calls it from the interrupt handler, on
// the exception stack with the interrupted g in the g register -- a stack
// check there is a guaranteed morestack-on-g0 fatal (it was). Everything
// here must share the contract: the atomics are nosplit asm, and the
// platform's goos.PreemptM hook must be nosplit too.
//
//go:nosplit
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
	gp := getg()
	if gp == nil {
		return 0, 0
	}
	mp := gp.m
	if mp == nil {
		return 0, 0
	}
	if mp.signalPending.Load() == 0 {
		return 0, 0
	}
	if !goos.AsyncPreempt || mp.gsignal == nil ||
		gp == mp.g0 || gp == mp.gsignal || mp.curg != gp {
		// The request cannot be served asynchronously (tier not armed, or
		// the moment is wrong), but it MUST still be consumed: preemptM
		// only delivers on a 0->1 transition, so a flag left set here
		// would silence every future IPI to this m. The cooperative
		// poison is already planted; acknowledging is correct and matches
		// doSigPreempt, which acknowledges every delivery.
		mp.preemptGen.Add(1)
		mp.signalPending.Store(0)
		return 0, 0
	}
	tamagoPreemptStats.accepted++
	dmScratchInc(0xb1d0)
	sg := mp.gsignal
	return uintptr(unsafe.Pointer(sg)), sg.stack.hi
}

// dmScratchInc/dmScratch32 mirror diagnostic counters into the platform's
// fixed post-mortem scratch (see the bmx board's postmortem.go): Go
// variables die with a watchdog reset (bss is cleared at boot), fixed
// scratch survives it and the next boot prints it. Bring-up diagnostics.
//
//go:nosplit
func dmScratchInc(addr uintptr) {
	*(*uint32)(unsafe.Pointer(addr))++
}

//go:nosplit
func dmScratch32(addr uintptr, v uint32) {
	*(*uint32)(unsafe.Pointer(addr)) = v
}

// tamagoPreemptStats counts the async tier's decision points, printed by
// the deadman: how many deliveries were accepted for the trap-frame path,
// how many interrupted PCs isAsyncSafePoint approved (frames actually
// rewritten into asyncPreempt), and how many it declined. Racy counters,
// diagnostic only.
var tamagoPreemptStats struct {
	accepted  uint32
	rewritten uint32
	unsafePC  uint32
}

// tamagoIRQAck lets the platform acknowledge self-contained interrupt
// sources (its periodic tick, its wake doorbell) inside the handler, so
// the relay-model core can return unmasked without involving the service
// goroutine. True means nothing is left pending: the handler skips the
// relay and does not re-mask. Nosplit: interrupt context, exception stack.
//
//go:linkname tamagoIRQAck
//go:nosplit
func tamagoIRQAck() bool {
	if goos.IRQAck == nil {
		return false
	}
	return goos.IRQAck()
}

// tamagoPreemptAck consumes a pending preemptM request on paths that never
// run the async tier (core 0's relay-model IRQ handler): the delivery did
// its job -- the interrupt was taken and the poison planted -- and the flag
// must clear or preemptM never rings this m's doorbell again.
//
//go:linkname tamagoPreemptAck
//go:nosplit
func tamagoPreemptAck() {
	gp := getg()
	if gp == nil {
		return
	}
	mp := gp.m
	if mp == nil {
		return
	}
	if mp.signalPending.Load() != 0 {
		mp.preemptGen.Add(1)
		mp.signalPending.Store(0)
	}
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
			tamagoPreemptStats.rewritten++
			dmScratchInc(0xb1d4)
			dmScratch32(0xb1e0, frame.pc)
			dmScratch32(0xb1e4, frame.sp)
			dmScratch32(0xb1e8, frame.lr)
			dmScratch32(0xb1ec, uint32(newpc))
			frame.pc = uint32(abi.FuncPCABI0(asyncPreempt))
		} else {
			tamagoPreemptStats.unsafePC++
			dmScratchInc(0xb1d8)
			// Decorrelate the retry. Doorbell-to-handler latency on this
			// hardware is nearly deterministic, so a retry loop (suspendG)
			// samples the SAME unsafe PC of a tight loop every time --
			// observed as ~10k consumed preemptions a second landing
			// nowhere. A pseudo-random dawdle before resuming shifts the
			// interrupted loop's phase against the next delivery; signal
			// jitter provides this for free on hosted systems.
			procyield(32 + uint32(nanotime())&0x1ff)
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
	if goos.SchedTick != nil {
		tamagoDeadmanCheck()
	}
	if goos.PreemptM != nil {
		tamagoTimeSlice()
	}

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

// tamagoTimeSlice is sysmon's time-slicing role, bare-metal style. There
// is no sysmon thread here, and only the interrupt-taking core's running
// goroutine gets the cooperative poison -- a core that takes no interrupts
// never yields, so any always-runnable goroutine on a secondary starves
// its P's peers forever (observed: six workers freezing the background
// sweeper, which runtime.GC waits on, before a single preemption was ever
// sent). Piggybacking on the interrupt-driven core's tick, every eighth
// invocation rings the doorbell of every OTHER M running a goroutine; the
// receiving handler plants the poison on its own core, and always-runnable
// goroutines get sliced at tens-of-milliseconds granularity everywhere.
var tamagoSliceN uint32

//go:nosplit
func tamagoTimeSlice() {
	tamagoSliceN++
	if tamagoSliceN&7 != 0 {
		return
	}
	var self *m
	if gp := getg(); gp != nil {
		self = gp.m
	}
	for mp := allm; mp != nil; mp = mp.alllink {
		if mp != self && mp.curg != nil {
			preemptM(mp)
		}
	}
}

// The deadman: a wedged scheduler silences every Go-level witness, but
// interrupts keep firing, and tamagoPreempt runs on each one. When the
// platform arms goos.SchedTick (pointing at a counter some ordinary
// goroutine advances), this watches for a long stretch of interrupts with
// no advance and then dumps scheduler state -- racily, lock-free, from
// interrupt context, because any lock may be held by the very core that
// died. Output goes through goos.RawPutc, never print: the print path
// takes locks a dead core can hold. A bring-up diagnostic.
var tamagoDeadman struct {
	n        uint32
	lastTick uint32
	lastAt   uint32 // RawTicks at the last tick advance (us-class)
	fires    uint32
}

//go:nosplit
func dmPuts(s string) {
	for i := 0; i < len(s); i++ {
		goos.RawPutc(s[i])
	}
}

//go:nosplit
func dmHex(v uint64) {
	const digits = "0123456789abcdef"
	dmPuts("0x")
	started := false
	for i := 60; i >= 0; i -= 4 {
		d := byte(v>>uint(i)) & 0xf
		if d != 0 || started || i == 0 {
			goos.RawPutc(digits[d])
			started = true
		}
	}
}

//go:nosplit
func tamagoDeadmanCheck() {
	d := &tamagoDeadman
	if goos.RawPutc == nil || goos.RawTicks == nil {
		return
	}
	// Sample every few entries, judge by TIME: interrupt counts race
	// ahead by orders of magnitude in a preemption storm, and the first
	// deadman false-fired off exactly that -- then its one-shot latch
	// spent the dump before the real silence arrived. Three fires, ten
	// seconds of no scheduler progress each.
	d.n++
	if d.n&63 != 0 {
		return
	}
	now := goos.RawTicks()
	tick := *goos.SchedTick
	if tick != d.lastTick {
		d.lastTick = tick
		d.lastAt = now
		return
	}
	if now-d.lastAt < 10_000_000 { // 10s at 1 MHz; uint32 wrap-safe
		return
	}
	if d.fires >= 3 {
		return
	}
	d.fires++
	d.lastAt = now // next fire no sooner than another window

	dmPuts("\n[deadman] scheduler starved; state dump (racy, lock-free):\n")
	dmPuts("[deadman] async accepted ")
	dmHex(uint64(tamagoPreemptStats.accepted))
	dmPuts(" rewritten ")
	dmHex(uint64(tamagoPreemptStats.rewritten))
	dmPuts(" unsafe ")
	dmHex(uint64(tamagoPreemptStats.unsafePC))
	dmPuts("\n")
	dmPuts("[deadman] sched.lock.key ")
	dmHex(uint64(sched.lock.key))
	dmPuts(" gomaxprocs ")
	dmHex(uint64(uint32(gomaxprocs)))
	dmPuts("\n")

	// Whether a stop-the-world is in progress, and how many Ps it is still
	// waiting for. This is the difference between "the world is stopped and
	// whoever stopped it is stuck" and "the world will not stop because
	// something will not yield", which have nothing in common but the symptom.
	dmPuts("[deadman] gcwaiting ")
	if sched.gcwaiting.Load() {
		dmPuts("1")
	} else {
		dmPuts("0")
	}
	dmPuts(" stopwait ")
	dmHex(uint64(uint32(sched.stopwait)))
	dmPuts(" npidle ")
	dmHex(uint64(sched.npidle.Load()))
	dmPuts("\n")

	for i := 0; i < len(allp); i++ {
		pp := allp[i]
		if pp == nil {
			continue
		}
		dmPuts("[deadman] P")
		dmHex(uint64(uint32(pp.id)))
		dmPuts(" status ")
		dmHex(uint64(pp.status))
		dmPuts(" m ")
		dmHex(uint64(uintptr(pp.m)))
		dmPuts(" runq ")
		dmHex(uint64((pp.runqtail - pp.runqhead) % uint32(len(pp.runq))))
		dmPuts("\n")
	}

	for mp := allm; mp != nil; mp = mp.alllink {
		dmPuts("[deadman] M")
		dmHex(uint64(uint32(mp.id)))
		dmPuts(" procid ")
		dmHex(mp.procid)
		dmPuts(" locks ")
		dmHex(uint64(uint32(mp.locks)))
		dmPuts(" spin ")
		if mp.spinning {
			dmPuts("1")
		} else {
			dmPuts("0")
		}
		dmPuts(" sema ")
		dmHex(uint64(mp.waitsemacount))
		dmPuts(" curg ")
		if mp.curg != nil {
			dmHex(uint64(mp.curg.goid))
		} else {
			dmPuts("-")
		}
		dmPuts("\n")

		// The goroutine that will not stop is the whole question, and its
		// number alone cannot answer it. Status says whether it is running or
		// already parked; the poison flags say whether a preemption request
		// even reached it; and the PC says WHERE, which is the one fact that
		// turns this from deduction into a lookup:
		//
		//	go tool addr2line build/kernel.elf <<< 0x<pc>
		//
		// Printed for every M with a g, because a wedge involving two is a
		// different animal from one involving one.
		if gp := mp.curg; gp != nil {
			dmPuts("[deadman]   g")
			dmHex(uint64(gp.goid))
			dmPuts(" status ")
			dmHex(uint64(gp.atomicstatus.Load()))
			dmPuts(" poisoned ")
			if gp.stackguard0 == stackPreempt {
				dmPuts("1")
			} else {
				dmPuts("0")
			}
			dmPuts(" preempt ")
			if gp.preempt {
				dmPuts("1")
			} else {
				dmPuts("0")
			}
			dmPuts(" preemptStop ")
			if gp.preemptStop {
				dmPuts("1")
			} else {
				dmPuts("0")
			}
			dmPuts(" pc ")
			dmHex(uint64(gp.sched.pc))
			dmPuts(" sp ")
			dmHex(uint64(gp.sched.sp))
			dmPuts("\n")
		}
	}
	if goos.DeadmanHook != nil {
		goos.DeadmanHook()
	}
	dmPuts("[deadman] end\n")
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
