package cyclebound

import (
	"strings"
	"testing"
)

// Keeping VSYNC/VBLANK alive across a JSR is what lets the ordinary two-sprite
// kernel — both players placed through one shared routine — be classified as the
// blank-time code it is. The rule has a dangerous direction: if it decides a
// callee cannot touch the display when it can, a region that really does turn the
// picture on is classified blank and skipped by the budget proof.
//
// litmus_jsr_display holds three routines of identical shape that differ only in
// one store, all called from the same place with the same index values:
//
//	SafeCall    sta COLUP0,x   x in {0,1} → $06/$07     must stay blank
//	PeriCall    sta VSYNC,x    x in {0,1} → $00/$01     must become visible
//	DirectCall  sta VBLANK     no index   → $01         must become visible
//
// A rule that answers the same way for all three is wrong whichever answer it
// gives, so they are asserted apart rather than one at a time. DirectCall is
// there so the rule cannot be satisfied by reasoning about indexed stores alone.
func TestJSRDisplayRuleSeparatesTheTwins(t *testing.T) {
	rep, err := Prove("../../roms/litmus/litmus_jsr_display.asm", DefaultBudget)
	if err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}

	kind := map[string]string{}
	// Lines holds every visible region and BlankLines every blank one, so the two
	// together are the complete table — reading only one would make an absent
	// routine look like the wrong classification.
	all := append(append([]Region{}, rep.Lines...), rep.BlankLines...)
	for _, r := range all {
		for _, name := range []string{"SafeCall", "PeriCall", "DirectCall"} {
			if strings.Contains(r.StartLoc, name) {
				kind[name] = r.Kind
			}
		}
	}
	for _, name := range []string{"SafeCall", "PeriCall", "DirectCall"} {
		if kind[name] == "" {
			t.Fatalf("premise broken: %s must open a WSYNC region; got %+v across %d regions",
				name, kind, len(all))
		}
	}

	if kind["SafeCall"] != "blank" {
		t.Errorf("SafeCall classified %q, want \"blank\": its indexed store reaches $06/$07 only, so "+
			"the display bits must survive the call. This is the shape every shared positioning "+
			"routine has; calling it visible reports a scanline tear where there is only "+
			"blank-time code.", kind["SafeCall"])
	}
	for _, name := range []string{"PeriCall", "DirectCall"} {
		if kind[name] != "visible" {
			t.Errorf("%s classified %q, want \"visible\": it can write VSYNC/VBLANK, so treating its "+
				"region as blank would skip the budget proof on code that turns the picture on — "+
				"the unsound direction.", name, kind[name])
		}
	}
}
