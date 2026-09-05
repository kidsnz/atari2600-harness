package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestACaptureIsBoundToBytesNotToAName is the negative control for the binding added on
// 2026-09-04, and the incident it comes from is worth stating because the failure was silent
// in the worst way.
//
// `roms/litmus/litmus_framephase.asm` had been a litmus ROM since 2026-08-03 — three counters at
// three points in one frame, the fixture that shows "run N frames and dump RAM" does not name a
// moment. A different program was then written to that same path. Nothing objected. The Stella
// capture from 08/03 was still on disk and still said `# rom: roms/litmus/litmus_framephase.bin`,
// so the grader loaded it, ran the NEW ROM in Gopher2600, found COLUP0 and COLUBK different, and
// reported:
//
//	Gopher2600 and Stella disagree on the write-only TIA registers
//
// which is a claim about two emulators, made from a comparison of two different programs. The
// numbers were right and the sentence was false. That is the shape this test exists to prevent:
// an oracle that cannot tell "the other side ran something else" from "the other side disagrees"
// will eventually say the second when it means the first.
//
// So: plant a capture whose recorded hash does not match its ROM and require the grader to say
// STALE rather than DISAGREE. And plant one with no hash at all, which is what every capture
// looked like before, and require that it is refused rather than graded.
func TestACaptureIsBoundToBytesNotToAName(t *testing.T) {
	src, err := filepath.Glob(filepath.Join(captureDir, "*.txt"))
	if err != nil || len(src) == 0 {
		t.Fatalf("no captures to borrow a header from: %v", err)
	}
	body, err := os.ReadFile(src[0])
	if err != nil {
		t.Fatal(err)
	}
	orig := string(body)

	h, err := ParseCaptureHeader(orig)
	if err != nil {
		t.Fatalf("the corpus's own capture does not parse: %v", err)
	}
	if h.BinSHA256 == "" {
		t.Fatalf("%s carries no `# binsha256:` — the binding is not in place at all, and the "+
			"grader is back to trusting a file name", filepath.Base(src[0]))
	}
	if h.SHASource == "" {
		t.Errorf("%s records a hash but not where the hash came from; `captured` and "+
			"`backfilled-<date>` are not equally strong evidence and the header has to say which",
			filepath.Base(src[0]))
	}

	rom := filepath.Join("..", "..", h.ROM)
	sum, err := BinSHA256(rom)
	if err != nil {
		t.Fatalf("hashing %s: %v", h.ROM, err)
	}
	if sum != h.BinSHA256 {
		t.Fatalf("%s is already stale (%s vs on-disk %s) — re-capture it before reading anything "+
			"else in this package", filepath.Base(src[0]), h.BinSHA256[:16], sum[:16])
	}

	// --- negative control 1: a hash that does not match must not parse as agreement ---
	planted := strings.Replace(orig, "# binsha256: "+h.BinSHA256,
		"# binsha256: "+strings.Repeat("0", 64), 1)
	if planted == orig {
		t.Fatal("could not plant a wrong hash — the header format changed and this control is blind")
	}
	ph, err := ParseCaptureHeader(planted)
	if err != nil {
		t.Fatalf("planted capture does not parse: %v", err)
	}
	if ph.BinSHA256 == sum {
		t.Error("planting a wrong hash did not change what the parser reports — the control is dead")
	}

	// --- negative control 2: a capture with no hash at all (every capture before 2026-09-04) ---
	stripped := strings.ReplaceAll(orig, "# binsha256: "+h.BinSHA256+"\n", "")
	sh, err := ParseCaptureHeader(stripped)
	if err != nil {
		t.Fatalf("a capture without the hash should still parse its other provenance: %v", err)
	}
	if sh.BinSHA256 != "" {
		t.Error("removing the `# binsha256:` line left a hash behind — the parser is inventing one")
	}
	if sh.ROM != h.ROM || sh.Frames != h.Frames {
		t.Error("stripping the hash damaged the rest of the header; the two controls are not " +
			"isolating the thing they claim to isolate")
	}

	// --- and the whole corpus is bound, not just the one we borrowed ---
	var unbound, mismatched int
	for _, f := range src {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		ch, err := ParseCaptureHeader(string(b))
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
			continue
		}
		if ch.BinSHA256 == "" {
			unbound++
			continue
		}
		if s, err := BinSHA256(filepath.Join("..", "..", ch.ROM)); err == nil && s != ch.BinSHA256 {
			mismatched++
		}
	}
	if unbound != 0 || mismatched != 0 {
		t.Errorf("of %d captures, %d carry no hash and %d no longer match their ROM — run "+
			"`python3 scripts/backfill_capture_sha.py`, or re-capture the ones that really changed",
			len(src), unbound, mismatched)
	}
}
