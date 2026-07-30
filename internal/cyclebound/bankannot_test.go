package cyclebound

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBankedImageReadsLineAnnotations shows that `@lines` works on a bank-switched
// kernel, which it did not before the per-bank source map.
//
// The reason it did not is worth stating exactly, because it hit EVERY bank and not
// just the ones people expected. DASM's listing address column is the physical ROM
// offset, so on an 8K image bank 0's rows sit below $1000 and srcmap.Parse discards
// them as TIA/RIOT equates, while bank 1's are stored under $1Fxx as if those were
// CPU addresses. A lookup at $F0xx therefore missed in both directions, `@lines`
// silently returned its default of 1, and every region on a banked ROM was budgeted
// at one scanline no matter what the source declared.
//
// No banked ROM in the corpus carries an annotation today, so nothing in the reports
// changed when this was fixed — which is exactly why the mechanism needs a witness
// of its own rather than an assertion that it now works.
func TestBankedImageReadsLineAnnotations(t *testing.T) {
	const src = "../../roms/techniques/banked_game.asm"
	body, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	lines := strings.Split(string(body), "\n")

	// The line that opens a region in bank 0, found by the prover itself rather than
	// hard-coded: KRow's `sta WSYNC`.
	target := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "KRow:") {
			target = i
			break
		}
	}
	if target < 0 {
		t.Skip("KRow label not found; the fixture has been rewritten")
	}

	run := func(t *testing.T, annotated bool) *Report {
		t.Helper()
		dir := t.TempDir()
		out := append([]string(nil), lines...)
		if annotated {
			out[target] += "   ; @lines 3"
		}
		path := filepath.Join(dir, "banked_annot.asm")
		if err := os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644); err != nil {
			t.Fatal(err)
		}
		rep, err := Prove(path, 76)
		if err != nil {
			t.Fatal(err)
		}
		if rep.BankedDeclined != "" {
			t.Fatalf("declined: %s", rep.BankedDeclined)
		}
		return rep
	}

	budgetAt := func(rep *Report, loc string) int {
		for _, r := range append(append([]Region(nil), rep.Lines...), rep.BlankLines...) {
			if strings.Contains(r.StartLoc, loc) {
				return r.Budget
			}
		}
		return -1
	}

	plain := run(t, false)
	if got := budgetAt(plain, "KRow"); got != 76 {
		t.Fatalf("unannotated KRow region budget = %d, want 76 (and want the region to be FOUND: "+
			"-1 means no region's start_loc names KRow, so this test is not looking at anything)", got)
	}

	annotated := run(t, true)
	if got := budgetAt(annotated, "KRow"); got != 3*76 {
		t.Errorf("`@lines 3` on a bank-switched image gave KRow a budget of %d, want %d — the "+
			"annotation is still being read through the flat address map", got, 3*76)
	}

	// And the report must say the annotations are readable, not repeat the old
	// "unavailable" note.
	if strings.Contains(annotated.SourceAnnotations, "unavailable") {
		t.Errorf("SourceAnnotations still says annotations are unavailable: %s", annotated.SourceAnnotations)
	}
	if !strings.Contains(annotated.SourceAnnotations, "resolved to source lines") {
		t.Errorf("SourceAnnotations does not report its coverage: %s", annotated.SourceAnnotations)
	}
	t.Logf("KRow budget %d -> %d with `@lines 3`; %s",
		budgetAt(plain, "KRow"), budgetAt(annotated, "KRow"), annotated.SourceAnnotations)
}
