package emu

// What the ENGINE does when a timer write lands near the RIOT counter's underflow,
// measured one CPU cycle at a time.
//
// Several places in this repository said, until 2026-10-13, that the embedded engine
// "does not reproduce" the timer-wrap write race, on the strength of reading
// `Timer.Update`. Reading the same
// function the other way gives the opposite answer: `Update` keeps the expired flag
// when the write arrives with ticksRemaining == 0 and INTIM == 0xff, and `Step`
// decrements once per cycle for as long as the flag stands. Two readings do not
// settle it, so this file runs it.
//
// It pins what was observed, in both directions — the offsets where the requested
// interval takes and the offsets where it does not. It says nothing about real
// hardware or about any other emulator; no measurement here can.
//
// HOW THE PROBE STAYS OUT OF THE MEASUREMENT. A bus read of INTIM changes the timer
// (`Step` clears the expired flag on it), so nothing here reads through the bus.
// State comes from `Emu.TimerState`, which calls `Timer.PeekState`: that returns the
// struct fields and `ChipRefer(...)`, and `RIOTMemory.ChipRefer` is a bare array
// index. The read signal `Step` reacts to is set only in `RIOTMemory.Read`, which the
// peek never calls. That is the argument from the source;
// TestTimerWrapWriteStateProbeDoesNotPerturb is the measurement of it, and it also
// shows that a bus read DOES change the outcome, so the distinction is not academic.
//
// The cartridges are built in the test, not assembled, so there is no fixture to go
// stale and the cycle each write lands on is counted rather than derived.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
	"github.com/jetsetilly/gopher2600/hardware/memory/memorymap"
)

const (
	twsTIM1T  = 0x0294
	twsTIM8T  = 0x0295
	twsTIM64T = 0x0296
	twsT1024T = 0x0297
	twsINTIM  = 0x0284
	// twsRAM is where the control run sends its store: same instruction, same four
	// cycles, but the timer never hears about it.
	twsRAM = 0x0080

	twsStoreVal = 40
)

// twsProbe is one synthetic cartridge: arm a timer, wait `pad` CPU cycles, store.
type twsProbe struct {
	armReg    uint16
	armVal    uint8
	pad       int
	storeReg  uint16
	readINTIM bool // follow the store with `lda INTIM` — a bus read, on purpose
}

// image assembles the cartridge by hand and returns the address of the store.
// Everything after the store is a slide of NOPs ending in a jump-to-self, none of
// which touches the RIOT, so whatever the timer does afterwards is its own doing.
func (p twsProbe) image(t *testing.T) (rom []byte, storePC uint16) {
	t.Helper()
	rom = make([]byte, 4096)
	for i := range rom {
		rom[i] = 0xEA // nop
	}
	pc := 0
	emit := func(b ...byte) {
		copy(rom[pc:], b)
		pc += len(b)
	}
	emit(0x78, 0xD8, 0xA2, 0xFF, 0x9A)            // sei / cld / ldx #$FF / txs
	emit(0xA9, p.armVal)                          // lda #armVal
	emit(0x8D, byte(p.armReg), byte(p.armReg>>8)) // sta armReg
	pad := p.pad
	if pad < 0 || pad == 1 {
		t.Fatalf("a pad of %d cycles cannot be built from 2- and 3-cycle instructions", pad)
	}
	if pad%2 == 1 {
		// `jmp` to the next instruction: three cycles and no data access at all.
		next := 0xF000 + pc + 3
		emit(0x4C, byte(next), byte(next>>8))
		pad -= 3
	}
	pc += pad / 2           // the slide is already NOPs, two cycles each
	emit(0xA9, twsStoreVal) // lda #40
	storePC = uint16(0xF000 + pc)
	emit(0x8D, byte(p.storeReg), byte(p.storeReg>>8)) // sta storeReg
	if p.readINTIM {
		emit(0xAD, byte(twsINTIM&0xFF), byte(twsINTIM>>8)) // lda INTIM
	}
	copy(rom[0x0FF0:], []byte{0x4C, 0xF0, 0xFF}) // jmp *
	rom[0x0FFC], rom[0x0FFD] = 0x00, 0xF0
	rom[0x0FFE], rom[0x0FFF] = 0x00, 0xF0
	return rom, storePC
}

func twsLoad(t *testing.T, rom []byte) *Emu {
	t.Helper()
	p := filepath.Join(t.TempDir(), "timerprobe.bin")
	if err := os.WriteFile(p, rom, 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(p); err != nil {
		t.Fatal(err)
	}
	return e
}

// twsStep runs one `VCS.Step` and calls fn at the end of every CPU cycle in it —
// after the RIOT has stepped, which is where the engine's own cycle callback sits.
// It reports whether an instruction was executed (as opposed to a WSYNC stall
// cycle), by the same RdyFlg-before-the-step convention `stepInstr` uses.
func twsStep(e *Emu, fn func()) (executed bool, err error) {
	ready := e.VCS.CPU.RdyFlg
	err = e.VCS.Step(func(isCycle bool) error {
		if isCycle {
			fn()
		}
		return nil
	})
	return ready, err
}

// twsRun is a per-cycle record of the timer. log[c] is the state at the end of CPU
// cycle c, counted from 1 at reset; log[0] is the state before any cycle.
type twsRun struct {
	log   []TimerState
	pre   int // last cycle before the store instruction begins
	write int // the store's final cycle, the one that puts the value on the bus
}

func twsTrace(t *testing.T, rom []byte, storePC uint16, after int) twsRun {
	t.Helper()
	e := twsLoad(t, rom)
	r := twsRun{log: []TimerState{e.TimerState()}}
	for r.write == 0 || len(r.log)-1 < r.write+after {
		before := len(r.log) - 1
		executed, err := twsStep(e, func() { r.log = append(r.log, e.TimerState()) })
		if err != nil {
			t.Fatal(err)
		}
		if executed && r.write == 0 && e.VCS.CPU.LastResult.Address == storePC {
			r.pre, r.write = before, len(r.log)-1
		}
		if len(r.log) > 200000 {
			t.Fatalf("the store at $%04X was never reached", storePC)
		}
	}
	return r
}

// wraps lists the cycles on which INTIM went 0x00 -> 0xff.
func (r twsRun) wraps() []int {
	var out []int
	for c := 1; c < len(r.log); c++ {
		if r.log[c-1].INTIM == 0x00 && r.log[c].INTIM == 0xff {
			out = append(out, c)
		}
	}
	return out
}

// rate classifies the `n` cycles after the write. "interval" means INTIM dropped
// exactly once per `div` cycles; "1/cycle" means it dropped on every one of them.
// Anything else is returned as a description, and the callers fail on it.
func (r twsRun) rate(div, n int) string {
	base := r.log[r.write].INTIM
	interval, perCycle := true, true
	for k := 1; k <= n; k++ {
		got := r.log[r.write+k].INTIM
		if got != base-uint8(k/div) {
			interval = false
		}
		if got != base-uint8(k) {
			perCycle = false
		}
	}
	switch {
	case interval && !perCycle:
		return "interval"
	case perCycle && !interval:
		return "1/cycle"
	}
	return fmt.Sprintf("NEITHER(interval=%v 1/cycle=%v)", interval, perCycle)
}

// firstChange is how many cycles after the write INTIM first differs.
func (r twsRun) firstChange() int {
	for k := 1; r.write+k < len(r.log); k++ {
		if r.log[r.write+k].INTIM != r.log[r.write].INTIM {
			return k
		}
	}
	return -1
}

type twsArm struct {
	name string
	reg  uint16
	div  int
	n    uint8
}

type twsCase struct {
	arm    twsArm
	offset int // store's write cycle minus the cycle the counter wraps on
}

// twsCases is the sweep. The offset is moved one CPU cycle at a time across the
// underflow, under three different dividers before it so the answer cannot be a
// property of one of them. +255..+259 is the same neighbourhood one lap of the
// 8-bit counter later. -600 and +100 are the far-from-any-wrap controls.
func twsCases() []twsCase {
	arms := []twsArm{
		{"TIM1T", twsTIM1T, 1, 20},
		{"TIM8T", twsTIM8T, 8, 3},
		{"TIM64T", twsTIM64T, 64, 1},
	}
	var offsets []int
	for o := -6; o <= 6; o++ {
		offsets = append(offsets, o)
	}
	offsets = append(offsets, 100, 255, 256, 257, 258, 259)

	out := []twsCase{{twsArm{"TIM64T", twsTIM64T, 64, 10}, -600}}
	for _, a := range arms {
		for _, o := range offsets {
			out = append(out, twsCase{a, o})
		}
	}
	return out
}

// pad turns an intended offset into cycles of padding. The arming store's write
// cycle is A; the counter wraps div*n cycles later; `lda #` plus `sta abs` put the
// second write six cycles after the padding ends. The sweep does not trust this
// arithmetic — it measures the offset on a control run and fails if they disagree.
func (c twsCase) pad() int { return c.offset + c.arm.div*int(c.arm.n) - 6 }

// twsFast is the one thing the sweep is here to pin: the offsets at which the engine
// was observed to ignore the requested interval and count once per cycle.
func twsFast(offset int) bool { return offset == 1 || offset == 257 }

func TestTimerWrapWriteStateSweep(t *testing.T) {
	stores := []struct {
		name string
		reg  uint16
		div  int
	}{
		{"TIM64T", twsTIM64T, 64},
		{"T1024T", twsT1024T, 1024},
	}
	// Long enough to see a 1024-cycle interval tick twice.
	const window = 2100

	for _, st := range stores {
		var tbl strings.Builder
		fmt.Fprintf(&tbl, "\nstore %s <- %d\n", st.name, twsStoreVal)
		fmt.Fprintf(&tbl, "%-10s %6s | %-7s %5s %6s | %5s %5s %5s %6s %6s | %s\n",
			"armed", "offset", "expired", "INTIM", "1stchg", "+64", "+128", "+192", "+1024", "+2048", "rate")
		fast, normal := 0, 0

		for _, c := range twsCases() {
			name := fmt.Sprintf("%s/%s=%d/%+d", st.name, c.arm.name, c.arm.n, c.offset)

			// THE CONTROL RUN: identical code, but the second store goes to RAM. It
			// tells where the wrap falls when nothing interferes with it, which the
			// real run cannot — a write that lands before the wrap prevents it.
			ctlROM, ctlPC := twsProbe{armReg: c.arm.reg, armVal: c.arm.n, pad: c.pad(), storeReg: twsRAM}.image(t)
			ctl := twsTrace(t, ctlROM, ctlPC, 1000)
			wraps := ctl.wraps()
			if len(wraps) == 0 {
				t.Fatalf("%s: the control run never wrapped, so there is nothing to be near", name)
			}
			if got := ctl.write - wraps[0]; got != c.offset {
				t.Fatalf("%s: the write lands %+d cycles from the wrap, not the %+d intended — "+
					"the padding arithmetic is wrong and every row would be mislabelled", name, got, c.offset)
			}

			rom, pc := twsProbe{armReg: c.arm.reg, armVal: c.arm.n, pad: c.pad(), storeReg: st.reg}.image(t)
			run := twsTrace(t, rom, pc, window)
			if run.write != ctl.write || pc != ctlPC {
				t.Fatalf("%s: the control and the real run disagree about where the store is "+
					"(cycle %d vs %d)", name, ctl.write, run.write)
			}
			post := run.log[run.write]

			// The write cycle is identified by the instruction's last cycle. Check that
			// against the timer itself: the value appears on that cycle and not before.
			if post.Divider != st.div || post.INTIM != twsStoreVal-1 {
				t.Errorf("%s: after the write cycle the timer reads divider=%d INTIM=%d, expected "+
					"divider=%d INTIM=%d — the cycle taken for the write is not the write",
					name, post.Divider, post.INTIM, st.div, twsStoreVal-1)
			}
			if prev := run.log[run.write-1]; prev.Divider == st.div && prev.INTIM == post.INTIM {
				t.Errorf("%s: the timer already held the new state one cycle earlier", name)
			}

			rate := run.rate(st.div, window)
			at := func(k int) uint8 { return run.log[run.write+k].INTIM }
			fmt.Fprintf(&tbl, "%-10s %+6d | %-7v %5d %6d | %5d %5d %5d %6d %6d | %s\n",
				fmt.Sprintf("%s=%d", c.arm.name, c.arm.n), c.offset, post.Expired, post.INTIM,
				run.firstChange(), at(64), at(128), at(192), at(1024), at(2048), rate)

			if twsFast(c.offset) {
				fast++
				if rate != "1/cycle" || !post.Expired || run.firstChange() != 1 {
					t.Errorf("%s: measured rate=%s expired=%v firstChange=%d; this offset was observed "+
						"to count once per cycle with the expired flag still set",
						name, rate, post.Expired, run.firstChange())
				}
				continue
			}
			normal++
			if rate != "interval" || post.Expired || run.firstChange() != st.div {
				t.Errorf("%s: measured rate=%s expired=%v firstChange=%d; this offset was observed to "+
					"take the requested interval (first change after %d cycles, flag clear)",
					name, rate, post.Expired, run.firstChange(), st.div)
			}
		}
		t.Log(tbl.String())
		t.Logf("store %s: %d offsets at the requested interval, %d at one per cycle", st.name, normal, fast)
		if fast == 0 || normal == 0 {
			t.Errorf("store %s: %d fast and %d normal rows — a sweep with only one outcome "+
				"cannot show that the classification discriminates", st.name, fast, normal)
		}
	}
}

// The peek must not be part of the experiment. Two runs of the same cartridge, one
// sampled on every cycle and one sampled exactly once at the end, have to arrive at
// the same state — on a row that counts fast and on a row that does not.
//
// The second half is the positive control for that: a single BUS read of INTIM during
// the fast stretch ends it. So a probe that read through the bus would have reported
// the requested interval at every offset, and the sweep above would have been wrong.
func TestTimerWrapWriteStateProbeDoesNotPerturb(t *testing.T) {
	arm := twsArm{"TIM8T", twsTIM8T, 8, 3}
	const after = 192

	for _, offset := range []int{1, 2} {
		c := twsCase{arm, offset}
		rom, pc := twsProbe{armReg: arm.reg, armVal: arm.n, pad: c.pad(), storeReg: twsTIM64T}.image(t)
		dense := twsTrace(t, rom, pc, after)

		// Same cartridge, but the timer is looked at ONCE.
		e := twsLoad(t, rom)
		var once TimerState
		cycle, target := 0, dense.write+after
		for cycle < target {
			if _, err := twsStep(e, func() {
				cycle++
				if cycle == target {
					once = e.TimerState()
				}
			}); err != nil {
				t.Fatal(err)
			}
		}
		if got := dense.log[target]; got != once {
			t.Errorf("offset %+d: sampled every cycle the timer ends at %+v, sampled once it ends "+
				"at %+v — the probe is changing what it measures", offset, got, once)
		}
		t.Logf("offset %+d: %d peeks and 1 peek agree at cycle +%d: %+v", offset, target+1, after, once)
	}

	c := twsCase{arm, 1}
	rom, pc := twsProbe{armReg: arm.reg, armVal: arm.n, pad: c.pad(), storeReg: twsTIM64T, readINTIM: true}.image(t)
	run := twsTrace(t, rom, pc, after)
	// `lda INTIM` is four cycles and reads on the last, so the read is at write+4.
	const read = 4
	before, at := run.log[run.write+read-1], run.log[run.write+read]
	if !before.Expired || at.Expired {
		t.Errorf("expired flag across the bus read: %v -> %v, expected set -> clear", before.Expired, at.Expired)
	}
	for k := 1; k <= read; k++ {
		if got, want := run.log[run.write+k].INTIM, uint8(twsStoreVal-1-k); got != want {
			t.Errorf("INTIM %d cycles after the write is %d, expected %d (one per cycle up to the read)", k, got, want)
		}
	}
	for k := read; k <= after; k++ {
		if got, want := run.log[run.write+k].INTIM, at.INTIM-uint8((k-read)/64); got != want {
			t.Fatalf("INTIM %d cycles after the write is %d, expected %d (one per 64 after the read)", k, got, want)
		}
	}
	t.Logf("a bus read of INTIM %d cycles into the fast stretch: INTIM %d -> %d, expired %v -> %v, "+
		"then one per 64 (INTIM %d at +%d)", read, before.INTIM, at.INTIM, before.Expired, at.Expired,
		run.log[run.write+after].INTIM, after)
}

// twsWrite is one store to a divider register seen while running a real ROM.
type twsWrite struct {
	pc       uint16
	reg      string
	scanline int
	pre      TimerState
	post     TimerState
	trail    []uint8 // INTIM on each following cycle, up to the next divider write
}

// twsWatchWrites runs one frame and records every divider write with the timer's
// state before the instruction, at the end of its write cycle, and on each of the
// next `n` cycles (fewer if another divider write comes first).
func twsWatchWrites(t *testing.T, rom string, n int) []twsWrite {
	t.Helper()
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(rom); err != nil {
		t.Skipf("ROM unavailable, nothing measured (%s): %v", rom, err)
	}
	if err := e.RunFrames(2); err != nil {
		t.Fatal(err)
	}
	var out []twsWrite
	open := false // the last entry of out is still collecting its trail
	start := e.Coords().Frame
	for e.Coords().Frame == start {
		pre := e.TimerState()
		executed, err := twsStep(e, func() {
			if open && len(out[len(out)-1].trail) < n {
				w := &out[len(out)-1]
				w.trail = append(w.trail, e.TimerState().INTIM)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		lr := e.VCS.CPU.LastResult
		if !executed || lr.Defn == nil || lr.Defn.Effect != instructions.Write {
			continue
		}
		canon, area := memorymap.MapAddress(lr.InstructionData, false)
		reg, ok := timerDividerRegs[canon]
		if area != memorymap.RIOT || !ok {
			continue
		}
		// The callback above ran during this very instruction, so its samples belong
		// to the cycles BEFORE this write; they were appended to the previous entry,
		// which is right. Close that one and open the new.
		out = append(out, twsWrite{pc: lr.Address, reg: reg, scanline: e.Coords().Scanline, pre: pre, post: e.TimerState()})
		open = true
	}
	return out
}

// Does the existing positive witness contain a write the engine reacts to? The
// detector's report and the engine's behaviour are put side by side, row by row.
func TestTimerWrapWriteStateOnTheExistingLitmus(t *testing.T) {
	const trail = 32

	detector := func(rom string) map[uint16]TimerDividerHazard {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(rom); err != nil {
			t.Skipf("ROM unavailable, nothing measured (%s): %v", rom, err)
		}
		if err := e.RunFrames(2); err != nil {
			t.Fatal(err)
		}
		hz, err := e.WatchTimerDividerHazard(1)
		if err != nil {
			t.Fatal(err)
		}
		m := map[uint16]TimerDividerHazard{}
		for _, h := range hz {
			m[h.PC] = h
		}
		return m
	}

	measure := func(rom string) (rows []string, fired, fast []int) {
		hz := detector(rom)
		var tbl strings.Builder
		fmt.Fprintf(&tbl, "\n%s\n%-4s %-6s %4s | %-28s %9s %-5s | %-7s %5s %5s | %s\n", rom,
			"row", "PC", "line", "before: INTIM/ticks/div/exp", "untilWrap", "fired", "expired", "INTIM", "+32", "rate")
		row := 0
		for _, w := range twsWatchWrites(t, rom, trail) {
			if w.reg != "T1024T" {
				continue // the arming stores; the hazard writes are all T1024T
			}
			row++
			if len(w.trail) < trail {
				t.Fatalf("%s row %d: only %d cycles before the next divider write, need %d",
					rom, row, len(w.trail), trail)
			}
			held, perCycle := true, true
			for k, got := range w.trail {
				if got != w.post.INTIM {
					held = false
				}
				if got != w.post.INTIM-uint8(k+1) {
					perCycle = false
				}
			}
			rate := "NEITHER"
			switch {
			case held && !perCycle:
				rate = "interval"
			case perCycle && !held:
				rate = "1/cycle"
				fast = append(fast, row)
			}
			h, f := hz[w.pc]
			until := "-" // the detector states a distance only on the rows it reports
			if f {
				fired = append(fired, row)
				until = fmt.Sprint(h.UntilWrap)
			}
			fmt.Fprintf(&tbl, "%-4d $%04X %4d | %5d /%5d /%5d /%-5v   %9s %-5v | %-7v %5d %5d | %s\n",
				row, w.pc, w.scanline, w.pre.INTIM, w.pre.TicksRemaining, w.pre.Divider, w.pre.Expired,
				until, f, w.post.Expired, w.post.INTIM, w.trail[trail-1], rate)
			if rate == "NEITHER" {
				t.Errorf("%s row %d: INTIM neither held nor fell once per cycle over %d cycles: %v",
					rom, row, trail, w.trail)
			}
			if (rate == "1/cycle") != w.post.Expired {
				t.Errorf("%s row %d: rate=%s but expired=%v after the write — in every other "+
					"measurement here the two go together", rom, row, rate, w.post.Expired)
			}
			rows = append(rows, rate)
		}
		t.Log(tbl.String())
		return rows, fired, fast
	}

	eq := func(a, b []int) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	// litmus_timerwrap_hit: twelve rows, N = 1..12 in order.
	rows, fired, fast := measure("../../roms/litmus/litmus_timerwrap_hit.bin")
	if len(rows) != 12 {
		t.Fatalf("litmus_timerwrap_hit: %d T1024T stores in a frame, the source has 12", len(rows))
	}
	if want := []int{3, 4, 5, 6, 9, 10}; !eq(fired, want) {
		t.Errorf("litmus_timerwrap_hit: the detector fired on rows %v, measured %v", fired, want)
	}
	if want := []int{5, 9}; !eq(fast, want) {
		t.Errorf("litmus_timerwrap_hit: the engine counted once per cycle after rows %v, measured %v", fast, want)
	}
	t.Logf("litmus_timerwrap_hit: detector fired on rows %v; engine ran at one per cycle after rows %v", fired, fast)

	// litmus_timerwrap_nearmiss: one T1024T store per frame.
	rows, fired, fast = measure("../../roms/litmus/litmus_timerwrap_nearmiss.bin")
	if len(rows) != 1 || len(fired) != 0 || len(fast) != 0 {
		t.Errorf("litmus_timerwrap_nearmiss: %d stores, detector fired on %v, engine fast after %v; "+
			"measured 1 store, neither", len(rows), fired, fast)
	}
}

// `WatchTimerDividerHazard` estimates the distance to the underflow from the state
// before the store and fires when that distance is inside the store's four cycles.
// This puts its verdict next to the distance MEASURED on the control run (which
// never asks the detector anything), and next to what the engine then did.
//
// Until 2026-10-13 the estimate was ticksRemaining + INTIM*divider in every state.
// That is the measured distance before the first wrap and is not after it, where the
// engine counts one per cycle and the distance is INTIM+1. The old formula is kept
// here as the `old` column — computed in this test, not by the detector — so the
// table shows what it would have said: silent on the two rows the engine answers
// with one-per-cycle counting one lap on (+257 armed with TIM8T and TIM64T), and
// firing at +255 armed with TIM1T, five cycles short of the wrap.
//
// What is pinned now: the detector fires exactly when the measured distance is
// within the store's four cycles, and reports that distance.
//
// It is a comparison against the ENGINE only. Whether the detector's window is the
// right one for hardware is not something this can say.
func TestTimerWrapWriteStateDetectorDistance(t *testing.T) {
	var tbl strings.Builder
	fmt.Fprintf(&tbl, "\nstore T1024T <- %d\n%-10s %6s | %-24s %8s %8s %-5s | %s\n", twsStoreVal,
		"armed", "offset", "before: INTIM/ticks/div/exp", "old", "measured", "fired", "engine")

	type key struct {
		arm    string
		offset int
	}
	fired := map[key]bool{}
	var agree, disagree, silentFast, firedNormal, oldSilentFast, oldFiredNormal int
	// `sta abs` is four cycles; the detector's window is the store's own length.
	const storeCycles = 4

	for _, c := range twsCases() {
		name := fmt.Sprintf("%s=%d/%+d", c.arm.name, c.arm.n, c.offset)
		ctlROM, ctlPC := twsProbe{armReg: c.arm.reg, armVal: c.arm.n, pad: c.pad(), storeReg: twsRAM}.image(t)
		ctl := twsTrace(t, ctlROM, ctlPC, 1000)
		measured := -1
		for _, w := range ctl.wraps() {
			if w > ctl.pre {
				measured = w - ctl.pre
				break
			}
		}
		if measured < 0 {
			t.Fatalf("%s: no wrap after the store in the control run", name)
		}

		rom, pc := twsProbe{armReg: c.arm.reg, armVal: c.arm.n, pad: c.pad(), storeReg: twsT1024T}.image(t)
		run := twsTrace(t, rom, pc, 64)
		pre := run.log[run.pre]
		if pre != ctl.log[ctl.pre] {
			t.Fatalf("%s: the control and the real run differ BEFORE the store (%+v vs %+v)",
				name, ctl.log[ctl.pre], pre)
		}
		old := pre.TicksRemaining + int(pre.INTIM)*pre.Divider
		engine := run.rate(1024, 64)

		hz, err := twsLoad(t, rom).WatchTimerDividerHazard(1)
		if err != nil {
			t.Fatal(err)
		}
		f := false
		for _, h := range hz {
			if h.PC != pc {
				continue
			}
			f = true
			if h.UntilWrap != measured {
				t.Errorf("%s: the detector reports untilWrap=%d, the control run measures %d "+
					"cycles from the same point to the wrap", name, h.UntilWrap, measured)
			}
			if h.StoreCycles != storeCycles {
				t.Errorf("%s: the detector reports a %d-cycle store, `sta abs` is %d",
					name, h.StoreCycles, storeCycles)
			}
		}
		fired[key{c.arm.name, c.offset}] = f

		// THE FIRING RULE, against a distance the detector had no part in producing.
		// These are the rows that fail if the post-wrap distance goes back to
		// INTIM*divider.
		if want := measured <= storeCycles; f != want {
			t.Errorf("%s: measured %d cycles to the wrap against a %d-cycle store, so the detector "+
				"should fire=%v; it fired=%v (engine: %s)", name, measured, storeCycles, want, f, engine)
		}

		fmt.Fprintf(&tbl, "%-10s %+6d | %5d /%5d /%5d /%-5v %8d %8d %-5v | %s\n",
			fmt.Sprintf("%s=%d", c.arm.name, c.arm.n), c.offset, pre.INTIM, pre.TicksRemaining,
			pre.Divider, pre.Expired, old, measured, f, engine)

		// THE DISTANCE. Before the first wrap ticksRemaining + INTIM*divider was
		// observed to be exact. After it the engine counts once per cycle whatever the
		// divider says, so the next 0x00 -> 0xff is INTIM+1 cycles away, with
		// ticksRemaining standing at 0.
		if !pre.Expired {
			agree++
			if old != measured {
				t.Errorf("%s: not yet wrapped, ticks+INTIM*divider = %d, measured %d — observed equal", name, old, measured)
			}
		} else {
			disagree++
			if want := int(pre.INTIM) + 1; measured != want || pre.TicksRemaining != 0 {
				t.Errorf("%s: already wrapped, measured %d cycles to the next wrap with ticks=%d, "+
					"observed INTIM+1 = %d with ticks=0", name, measured, pre.TicksRemaining, want)
			}
		}

		// THE VERDICT against what the engine did.
		if (engine == "1/cycle") != twsFast(c.offset) {
			t.Errorf("%s: engine rate %s disagrees with the sweep", name, engine)
		}
		switch {
		case twsFast(c.offset) && !f:
			silentFast++
		case !twsFast(c.offset) && f:
			firedNormal++
		}
		// The same two counts for the formula the detector used before, for the record.
		switch oldF := old >= 0 && old <= storeCycles; {
		case twsFast(c.offset) && !oldF:
			oldSilentFast++
		case !twsFast(c.offset) && oldF:
			oldFiredNormal++
		}
	}
	t.Log(tbl.String())

	// What was observed, pinned. Around the first wrap the detector fires on offsets
	// 0..+3 under every arming divider: one of the four is the offset the engine
	// reacts to (+1), the other three are not.
	for _, arm := range []string{"TIM1T", "TIM8T", "TIM64T"} {
		for o := -6; o <= 6; o++ {
			if want := o >= 0 && o <= 3; fired[key{arm, o}] != want {
				t.Errorf("armed %s, offset %+d: detector fired=%v, observed %v", arm, o, fired[key{arm, o}], want)
			}
		}
	}
	// One lap later the engine reacts again at +257, and the detector's window is the
	// same four offsets around it whatever the divider was before the wrap: by then
	// the engine is counting one per cycle under all three. (With INTIM*divider it
	// fired at +255..+259 armed with TIM1T and at +259 alone armed with TIM8T or
	// TIM64T.) +255 is five cycles short of the wrap and +100 is far from it; both
	// stay silent.
	for _, arm := range []string{"TIM1T", "TIM8T", "TIM64T"} {
		var got []int
		for _, o := range []int{100, 255, 256, 257, 258, 259} {
			if fired[key{arm, o}] {
				got = append(got, o)
			}
		}
		if want := []int{256, 257, 258, 259}; fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("armed %s: one lap on, the detector fired at offsets %v, observed %v", arm, got, want)
		}
	}
	t.Logf("ticks+INTIM*divider equals the measured distance on %d rows (not yet wrapped); on %d rows "+
		"(already wrapped) the measured distance is INTIM+1; engine fast and detector silent on %d rows "+
		"(%d with the old formula); detector fired and engine normal on %d (%d with the old formula)",
		agree, disagree, silentFast, oldSilentFast, firedNormal, oldFiredNormal)
	// The detector reports a four-cycle window and the engine reacts to one cycle of
	// it, at each of two laps under three arming dividers: 6 rows fast, all reported,
	// and 3 x 6 = 18 rows reported where the engine takes the requested interval.
	if silentFast != 0 || firedNormal != 18 {
		t.Errorf("engine fast with the detector silent on %d rows and detector fired with the engine "+
			"normal on %d; observed 0 and 18", silentFast, firedNormal)
	}
	if oldSilentFast != 2 || oldFiredNormal != 15 {
		t.Errorf("the formula used before gives %d silent-on-fast and %d fired-on-normal rows; "+
			"observed 2 and 15 when it was the detector's", oldSilentFast, oldFiredNormal)
	}
}
