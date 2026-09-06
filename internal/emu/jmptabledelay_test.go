package emu

import "testing"

// TestJumpTableGivesSingleCycleDelays settles a question the mailing list asked in 1998.
//
// Spending an ODD number of cycles is awkward on this machine: `ds N,$EA` can only make even
// delays and DASM's `SLEEP <odd>` reaches for `nop $00` or `bit $00`, both of which READ $00 —
// measured in `internal/emu/oddsleep_test.go`. Jim Nitchals posted a third answer on 1998-03-18,
// a jump table with *"single cycle resolution without the use of the carry flag"* built out of
// opcodes that eat the byte after them 〔stella-list `199803/msg00160`〕. Erik Mooney replied the
// same day: *"That's... incredibly wizardly. But isn't something backwards? … won't a larger value
// in the accumulator cause it to jump farther into the table, skipping more instructions and
// delaying fewer cycles?"* 〔`msg00161`〕, and Chris Wilkson confirmed it within the hour: *"Yeah,
// but mine was like that too...the delay equaled the max count minus the accum[ulator]"*
// 〔`msg00164`〕 — two people had built it independently before it was posted.
//
// ★So the reading is settled and the measurement was not. `roms/litmus/litmus_jmptable_delay.asm`
// enters the table at five successive offsets. Both claims hold exactly: successive entry points
// differ by ONE cycle, and the deeper the entry the SHORTER the delay.
//
// ★★Every entry pays the same 10 cycles of scaffolding — `jmp (indjmp)` is 5 and the table's
// closing `jmp (RetVec)` is 5 — so the constant is subtracted rather than assumed, and the empty
// interval at $8F is measured rather than hard-coded. What is left is the table's own cost.
//
// ★★★Why it matters here: the works' generated kernels time every write from the top of the line,
// which is why a repeat pass replaces `sta HMOVE` with `bit $80` rather than dropping it (see the
// multiple-HMOVE row in `docs/known-traps.md`). A delay that can be dialled to a single cycle, with
// no flag side effect and no address touched, is the tool that situation keeps asking for.
func TestJumpTableGivesSingleCycleDelays(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_jmptable_delay.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(30); err != nil {
		t.Fatal(err)
	}
	r, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}

	// The empty interval, measured. A hard-coded baseline would be asserting this harness's own
	// timing rather than the table's.
	base := int(r[0x0F])
	if base < 100 || base > 200 {
		t.Fatalf("the empty interval reads %d, outside anything this loop could take — the timer "+
			"setup has moved and every difference below is meaningless", base)
	}

	// `jmp (indjmp)` to enter plus `jmp (RetVec)` to leave: five cycles each, paid identically by
	// every entry.
	const scaffolding = 10

	type row struct {
		addr int
		name string
		want int
	}
	rows := []row{
		{0x00, "CmpZp-3", 6},
		{0x01, "CmpZp-2", 5},
		{0x02, "CmpZp-1", 4},
		{0x03, "CmpZp ($C5, CMP zero page — swallows the $EA as its address)", 3},
		{0x04, "NopByte ($EA, the bare nop)", 2},
	}

	got := make([]int, len(rows))
	for i, c := range rows {
		got[i] = int(r[c.addr]) - base - scaffolding
		if got[i] != c.want {
			t.Errorf("entry %s costs %d cycles, want %d (raw $%02X, empty interval %d, scaffolding %d)",
				c.name, got[i], c.want, r[c.addr], base, scaffolding)
		}
	}

	// ★The two claims, asserted as claims rather than as five numbers — so a change that keeps the
	// numbers but breaks the property is still caught, and vice versa.
	for i := 1; i < len(got); i++ {
		if got[i-1]-got[i] != 1 {
			t.Errorf("entries %s and %s differ by %d cycles, not 1 — the table no longer has "+
				"single-cycle resolution, which is the whole reason to prefer it over SLEEP",
				rows[i-1].name, rows[i].name, got[i-1]-got[i])
		}
	}
	if got[0] <= got[len(got)-1] {
		t.Errorf("the shallowest entry costs %d and the deepest %d — the sense is no longer "+
			"inverted, and Wilkson's \"the delay equaled the max count minus the accumulator\" "+
			"would no longer describe it", got[0], got[len(got)-1])
	}
	t.Logf("entry costs, scaffolding removed: %v (empty interval %d)", got, base)
}
