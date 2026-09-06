package build

import (
	"os"
	"path/filepath"
	"testing"
)

// TestROMBytesUsedBracketsAKnownSize calibrates the bracket against a ROM whose size is known by
// construction, because two earlier versions of this measurement were plausible and wrong and only
// a known answer separated them (see `ROMBytesUsed`).
//
// The fixture is 7 bytes of code, a 100-byte table and 4 vector bytes: **111**.
func TestROMBytesUsedBracketsAKnownSize(t *testing.T) {
	dir := t.TempDir()
	asm := filepath.Join(dir, "known.asm")
	src := "" +
		"        processor 6502\n" +
		"        org $F000\n" +
		"Start:  jmp Start\n" + // 3
		"        lda #$00\n" + // 2
		"        sta $80\n" + // 2   = 7
		"Table:  ds 100, $A5\n" + // 100
		"        org $FFFC\n" +
		"        .word Start\n" +
		"        .word Start\n" // 4
	if err := os.WriteFile(asm, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "known.bin")
	if out, err := Assemble(asm, bin); err != nil {
		t.Fatalf("assemble: %s", out)
	}

	used, trail, capacity, err := ROMBytesUsed(bin)
	if err != nil {
		t.Fatal(err)
	}
	const known = 111
	if used > known {
		t.Errorf("`used` is %d for a ROM that is %d bytes by construction — it is supposed to be a "+
			"LOWER bound and it has overstated, which means it is not a bound", used, known)
	}
	// ★This fixture emits no $FF of its own, so the bound is not merely a bound here: it is the
	// answer. If that stops being true the counting has changed, not the ROM.
	if used != known {
		t.Errorf("`used` is %d, want exactly %d — this fixture emits no $FF, so every non-fill "+
			"byte is one the program wrote", used, known)
	}
	if capacity != 4096 {
		t.Errorf("capacity %d, want 4096", capacity)
	}
	t.Logf("known 111 → used %d, trailing fill %d of %d", used, trail, capacity)

	// ★★And the width is the warning, so it must actually widen on a ROM full of $FF. Without
	// this, a caller could read every bracket as tight.
	wide := filepath.Join(dir, "wide.asm")
	if err := os.WriteFile(wide, []byte(""+
		"        processor 6502\n        org $F000\n"+
		"Start:  jmp Start\n"+
		"Solid:  ds 200, $FF\n"+ // 200 bytes the program wrote, indistinguishable from fill
		"        org $FFFC\n        .word Start\n        .word Start\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wbin := filepath.Join(dir, "wide.bin")
	if out, err := Assemble(wide, wbin); err != nil {
		t.Fatalf("assemble: %s", out)
	}
	wused, wtrail, _, err := ROMBytesUsed(wbin)
	if err != nil {
		t.Fatal(err)
	}
	const wideTrue = 3 + 200 + 4
	// ★The bound must still be a bound on a ROM built to defeat it.
	if wused > wideTrue {
		t.Errorf("`used` is %d for a ROM that is %d bytes by construction — the bound has "+
			"overstated on exactly the case it was warned about", wused, wideTrue)
	}
	// ★★And the understatement must be visible. This ROM's 200 bytes of deliberate $FF are
	// indistinguishable from fill, so `used` misses them and `trailingFF` swallows them; a caller
	// comparing the two sees a ROM that "uses 7 bytes" and "has 4083 free", which cannot both be
	// about a 207-byte program. That contradiction is the signal, and an earlier version of this
	// function hid it by presenting the pair as a bracket — a bracket that excluded the answer.
	if wused >= wideTrue-100 {
		t.Errorf("`used` is %d on a ROM whose data is 200 bytes of $FF; it should be far short of "+
			"%d, and if it is not then $FF is no longer the fill byte and this whole measure needs "+
			"re-deriving", wused, wideTrue)
	}
	t.Logf("200 bytes of deliberate $FF (true size %d) → used %d, trailing fill %d — the two "+
		"disagree, which is the warning", wideTrue, wused, wtrail)
}
