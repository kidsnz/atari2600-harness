package emu

import "testing"

// restrobeBands is the contract with scripts/gen_litmus_restrobe.py: same spacings, same k, same
// order. `want` is how many copies of P0 the machine actually draws on that band's one line.
//
// THE POINT OF THE TABLE IS THE ROWS THAT DISAGREE WITH EACH OTHER. The first version of this
// fixture measured spacing 8 alone, found 3+k, and the technique doc generalised it to "any spacing
// at or above three cycles works". Three of the eight spacings here say otherwise, and none of them
// is an edge case:
//
//	3, 5   FLAT at four. Every strobe after the first buys nothing at all.
//	6..8   3+k, the published ladder.
//	10     3+k until the copies start running off the right-hand edge, then it falls.
//	12     FASTER than 3+k, because at that spacing the old base's third copy has time to draw
//	       before the next strobe cancels it.
//
// A ladder measured at one spacing cannot tell a law from a coincidence. That is the rule
// check_instruments.py enforces for measurement functions; this fixture was not obeying it itself.
var restrobeBands = []struct {
	tag  string
	want int
}{
	{"s3 k1", 4}, {"s3 k2", 4}, {"s3 k3", 4}, {"s3 k4", 4}, {"s3 k5", 4},
	{"s4 k1", 4},
	{"s5 k1", 4}, {"s5 k2", 4}, {"s5 k3", 4}, {"s5 k4", 4}, {"s5 k5", 4},
	{"s6 k1", 4}, {"s6 k2", 5}, {"s6 k3", 6}, {"s6 k4", 7}, {"s6 k5", 8},
	{"s7 k1", 4}, {"s7 k2", 5}, {"s7 k3", 6}, {"s7 k4", 7}, {"s7 k5", 8},
	{"s8 k1", 4}, {"s8 k2", 5}, {"s8 k3", 6}, {"s8 k4", 7}, {"s8 k5", 8},
	{"s10 k1", 4}, {"s10 k2", 5}, {"s10 k3", 6}, {"s10 k4", 7}, {"s10 k5", 6},
	{"s12 k1", 4}, {"s12 k2", 6}, {"s12 k3", 8}, {"s12 k4", 9},
}

// TestRestrobeAddsCopies machine-locks what a mid-line RESP strobe is worth when the player is in a
// COPY mode: it does not move the copies already drawn, it ADDS another run of them -- but how many
// it adds depends on the SPACING between strobes, which is what restrobeBands sweeps.
// Grades roms/litmus/litmus_restrobe.bin (built by scripts/gen_litmus_restrobe.py); the rules are
// stated in docs/techniques/restrobe-copies.md.
//
// WHY THIS EXISTS. reference/atariage/180632 records solidcorp's 32-character display and files the
// mechanism as candidate ⑨, unverified. The work that prompted this had concluded "two players,
// three copies each, six shaped slots a scanline" was a hardware ceiling. It is eight per player at
// the right spacing, and sixteen for two -- the last band measures that rather than doubling eight
// in the head, which is how the doc used to state it.
//
// The three probes that had said otherwise were built the same wrong way: the claim is about a copy
// SEQUENCE restarting, and each of them measured a player with ONE copy, where "the first copy" and
// "the only copy" are the same thing -- so sprite-placement.md rule 8 ate the result every time.
// roms/litmus/litmus_resp_edge.asm had already pinned that contrast for RESBL vs RESPx; this fixture
// is the copy-mode half of the same question.
//
// NOTHING IN sprite-placement.md WAS WRONG, and this test nearly "corrected" it. The measurement
// that produced the ladder labelled its strobes one cycle high -- it padded to the store's FIRST
// cycle and called that the write cycle -- so every landing read three pixels off and rule 1 looked
// like it said 3c-60 where the machine said 3c-63. The catalogue's convention is the store's LAST
// cycle (scripts/gen_litmus_sprite_place.py:strobe pads to want-2); the fixture uses the same one,
// and rule 1 reproduces exactly. Two numbers that differ by one cycle are the same measurement
// until the origin is checked.
//
// HOW A BAND IS FOUND. Not by scanline number -- fixing band k to a line number graded the wrong
// rows once already and the failure looked like a hardware disagreement. The fixture puts an
// untouched SENTINEL line, the parked three at 3, 19, 35, in front of every measurement, so the
// grader anchors on the first sentinel it sees and then steps by the fixture's own three-line
// stride, re-checking the sentinel each time. A wrong anchor fails on the very next band.
func TestRestrobeAddsCopies(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_restrobe.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	if err := e.RunFrames(4); err != nil {
		t.Fatal(err)
	}

	// on returns where the named object was drawn on a scanline, left to right.
	on := func(line int, el string) []int {
		rs, _, err := e.DecomposeRow(line)
		if err != nil {
			return nil
		}
		var out []int
		for _, r := range rs {
			if r.Element == el {
				out = append(out, r.Clock)
			}
		}
		return out
	}
	isSentinel := func(line int) bool {
		xs := on(line, "P0")
		return len(xs) == 3 && xs[0] == 3 && xs[1] == 19 && xs[2] == 35
	}

	// THE FIRST SENTINEL-LOOKING LINE IS NOT NECESSARILY THE SENTINEL. A band's re-park strobe runs
	// in blank, so that line still shows the PREVIOUS band's base -- and for the first band the
	// previous base is the initial park, which makes the re-park line read 3, 19, 35 as well. So the
	// anchor is not "the first match", it is the only offset at which EVERY sentinel checks out.
	const stride = 3
	anchor := -1
	for cand := 20; cand < 200 && anchor < 0; cand++ {
		ok := true
		for i := 0; i <= len(restrobeBands) && ok; i++ {
			ok = isSentinel(cand + stride*i)
		}
		if ok {
			anchor = cand
		}
	}
	if anchor < 0 {
		t.Fatal("no offset makes every band's sentinel land on 3, 19, 35 — the fixture did not " +
			"park, or its three-line stride has changed")
	}
	for i, band := range restrobeBands {
		sent := anchor + stride*i
		if !isSentinel(sent) {
			t.Fatalf("band %s: line %d should be the sentinel (3, 19, 35) but draws %v — the "+
				"fixture's band order and this table have drifted apart",
				band.tag, sent, on(sent, "P0"))
		}
		xs := on(sent+1, "P0")
		t.Logf("%-7s line %3d: %d copies %v", band.tag, sent+1, len(xs), xs)
		if len(xs) != band.want {
			t.Errorf("band %s draws %d copies, want %d: %v", band.tag, len(xs), band.want, xs)
		}
		if len(xs) >= 2 && (xs[0] != 3 || xs[1] != 19) {
			// The first two copies are drawn before any strobe can reach them, so they never move.
			t.Errorf("band %s leads with %v, want 3 and 19 — a strobe reached backwards",
				band.tag, xs[:2])
		}
	}

	// WHERE AN ADDED COPY CAN LAND IS ALSO SPACING-DEPENDENT, and the doc used to state one half of
	// this as a law that "binds every kernel". At spacing 8 a strobe base is 3c-60 and the copies
	// NUSIZ makes from it are +16 and +32, so they land at x = 1 and 2 (mod 3) and a row needing a
	// letter at x = 0 (mod 3) cannot get it from a re-strobe. At spacing 6 the base ITSELF draws and
	// every added copy is a multiple of three. Both are witnessed, because a rule with only its
	// confirming case measured is the shape of the mistake this file is correcting.
	grid := func(tag string) []int {
		for i, b := range restrobeBands {
			if b.tag == tag {
				return on(anchor+stride*i+1, "P0")
			}
		}
		t.Fatalf("no band %s", tag)
		return nil
	}
	for _, x := range grid("s8 k5")[2:] {
		if x%3 == 0 {
			t.Errorf("spacing 8: added copy at x=%d is a multiple of three", x)
		}
	}
	sawMultipleOfThree := false
	for _, x := range grid("s6 k5")[2:] {
		if x%3 == 0 {
			sawMultipleOfThree = true
		}
	}
	if !sawMultipleOfThree {
		t.Error("spacing 6: no added copy is a multiple of three, so the two spacings no longer " +
			"disagree and the doc's spacing-dependence claim has lost its witness")
	}

	// TWO PLAYERS, MEASURED RATHER THAN DOUBLED. The doc opens by claiming two players reach sixteen
	// slots. Only one player had ever been measured; sixteen was 8x2 done in the head, and the same
	// doc's only real-world datapoint is 36char.bin at EIGHT slots a scanline. This is the band that
	// settles it. The two players do NOT simply add: their strobes share one 76-cycle line.
	two := anchor + stride*len(restrobeBands)
	if !isSentinel(two) {
		t.Fatalf("the two-player band's sentinel is missing at line %d: %v", two, on(two, "P0"))
	}
	a, b := on(two+1, "P0"), on(two+1, "P1")
	t.Logf("two players line %3d: P0 %d %v / P1 %d %v", two+1, len(a), a, len(b), b)
	if len(a)+len(b) != 16 {
		t.Errorf("two players draw %d slots (P0 %d, P1 %d), want 16: %v %v",
			len(a)+len(b), len(a), len(b), a, b)
	}
	seen := map[int]bool{}
	for _, x := range append(append([]int{}, a...), b...) {
		if seen[x] {
			t.Errorf("two players put two objects at x=%d, so the sixteen are not sixteen places", x)
		}
		seen[x] = true
	}
}
