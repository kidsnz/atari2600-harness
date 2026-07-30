package cyclebound

import (
	"testing"

	"github.com/jetsetilly/gopher2600/hardware/cpu/instructions"
)

// TestPageSensitiveTableIsWhatTheCostingAssumes pins the premises the page-cross
// costing rests on. They are premises about the ENGINE's opcode table, not about
// this package, so nothing here would notice them changing — and one bug in this
// area (pagePenalty asking whether the target RANGE straddled a page instead of
// whether the target left the base's page) has already cost four uncharged cycles
// per crossing read.
//
// Measured 2026-07-30 over all 256 opcodes:
//   - 32 indexed opcodes are PageSensitive, and every one of them is a READ.
//   - No WRITE is PageSensitive. On the 6502 `sta abs,X` is 5 cycles whether or not
//     it crosses; marking it would over-charge (safe, but wrong).
//   - Every indexed opcode that is NOT PageSensitive is explained: zero-page indexed
//     (cannot leave page 0), a write, or read-modify-write (which always pays the
//     extra cycle, so there is no conditional penalty to model).
//   - All 8 relative branches are PageSensitive, and pagePenalty returns 0 for them,
//     because their +1 is charged on the CFG edge instead. If it stopped excluding
//     them they would be charged twice.
func TestPageSensitiveTableIsWhatTheCostingAssumes(t *testing.T) {
	indexed := func(m instructions.AddressingMode) bool {
		return m == instructions.AbsoluteX || m == instructions.AbsoluteY || m == instructions.PostIndexed
	}

	sensitive, branches := 0, 0
	for op := 0; op < 256; op++ {
		d := instructions.Definitions[byte(op)]
		if d.Cycles == 0 {
			continue
		}
		if d.IsBranch() {
			branches++
			if !d.PageSensitive {
				t.Errorf("op %02X %v is a branch but not PageSensitive — a taken branch into another "+
					"page costs +1 and the CFG edge decides that from this flag's siblings", op, d.Operator)
			}
			// pagePenalty must keep its hands off: the edge already charges it.
			if p := (Instr{Def: d}).pagePenalty(topState()); p != 0 {
				t.Errorf("op %02X %v: pagePenalty returned %d for a branch — the CFG edge already "+
					"charges the taken-and-crossing cycle, so this would double it", op, d.Operator, p)
			}
			continue
		}
		if !indexed(d.AddressingMode) {
			if d.PageSensitive {
				t.Errorf("op %02X %v %v is PageSensitive without being indexed or a branch",
					op, d.Operator, d.AddressingMode)
			}
			continue
		}

		if d.PageSensitive {
			sensitive++
			if d.Effect != instructions.Read {
				t.Errorf("op %02X %v %v is PageSensitive but its effect is %v — only READS pay the "+
					"conditional cycle; writes and read-modify-writes are fixed-cost",
					op, d.Operator, d.AddressingMode, d.Effect)
			}
			continue
		}

		// Not sensitive: it has to be for one of three reasons, or the table has
		// stopped matching what the costing assumes.
		zeroPage := d.Bytes == 2 && d.AddressingMode != instructions.PostIndexed
		write := d.Effect == instructions.Write
		rmw := d.Effect != instructions.Read && d.Effect != instructions.Write
		if !zeroPage && !write && !rmw {
			t.Errorf("op %02X %v %v (bytes=%d cyc=%d eff=%v) is indexed and NOT PageSensitive, and it "+
				"is neither zero-page, a write, nor read-modify-write — an indexed READ that can cross "+
				"a page and is charged nothing for it", op, d.Operator, d.AddressingMode, d.Bytes,
				d.Cycles, d.Effect)
		}
	}

	if sensitive != 32 {
		t.Errorf("%d indexed opcodes are PageSensitive, measured 32. A drop means reads stopped being "+
			"charged; a rise means something that is not a read started being", sensitive)
	}
	if branches != 8 {
		t.Errorf("%d relative branches found, expected 8", branches)
	}
	t.Logf("256 opcodes: %d page-sensitive indexed reads, %d branches, every other indexed opcode "+
		"explained as zero-page / write / read-modify-write", sensitive, branches)
}
