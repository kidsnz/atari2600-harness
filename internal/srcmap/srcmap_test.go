package srcmap

import "testing"

const lstFix = `     59  f01c		       85 26		      sta	VDELP1
     60  f01e		       a9 fe		      lda	#>Font
     61  f020		       85 91		      sta	p0+1
      7  0006					COLUP0	equ $06
`

const symFix = `--- Symbol List (sorted by symbol)
Clr                      f007              (R )
COLUP0                   0006              (R )
Start                    f000
VBwait                   f01c
`

func TestParseAndLocate(t *testing.T) {
	m := Parse(lstFix, symFix, "/tmp/demo.asm")
	if got := m.Locate(0xF01C); got != "VBwait (demo.asm:59)" {
		t.Errorf("exact label: %q", got)
	}
	if got := m.Locate(0xF01E); got != "VBwait+2 (demo.asm:60)" {
		t.Errorf("label+off: %q", got)
	}
	// intermediate address absent from the listing → label+off only
	if got := m.Locate(0xF01F); got != "VBwait+3" {
		t.Errorf("label only: %q", got)
	}
	// an equ (below $1000) never becomes a label
	if got := m.Locate(0x0006); got != "" {
		t.Errorf("equ leaked: %q", got)
	}
	var nilMap *Map
	if nilMap.Locate(0xF000) != "" {
		t.Error("nil map should return empty")
	}
}

func TestSymbol(t *testing.T) {
	m := Parse(lstFix, symFix, "/tmp/demo.asm")
	if a, ok := m.Symbol("VBwait"); !ok || a != 0xF01C {
		t.Errorf("Symbol(VBwait) = %04X, %v", a, ok)
	}
	if a, ok := m.Symbol("Start"); !ok || a != 0xF000 {
		t.Errorf("Symbol(Start) = %04X, %v", a, ok)
	}
	// equs (below $1000, RAM/TIA) resolve too (for profile_line_budget's watch symbols; contract change 2026-07-12).
	// A patch to an address outside ROM is rejected by applyTempPatch's bounds check = safety is guaranteed there.
	if a, ok := m.Symbol("COLUP0"); !ok || a != 0x0006 {
		t.Errorf("Symbol(COLUP0) = %04X, %v — RAM/TIA equates must resolve for watch", a, ok)
	}
	if _, ok := m.Symbol("Nope"); ok {
		t.Error("Symbol(Nope) should not resolve")
	}
	var nilMap *Map
	if _, ok := nilMap.Symbol("X"); ok {
		t.Error("nil map Symbol should be false")
	}
}
