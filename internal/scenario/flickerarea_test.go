package scenario

import (
	"strings"
	"testing"
)

// TestMaxFlickerAreaGatesElementsNotColours covers the three directions of `max_flicker_area`.
//
// The check counts pixels whose DRAWING OBJECT differs between two consecutive frames. It exists
// because the archive judges flicker by AREA rather than by object count — *"an area as large as an
// Arkanoid wall is going to be hard on the eyes even at 30 Hz flicker, and positively
// headache-inducing beyond that"* 〔stella-list `200108/msg00315`, Erik Mooney, 2001-08-20〕 — and
// nothing here could measure area. The threshold in that sentence is a phrase, not a number, so
// this check invents none: the author sets a ceiling once, having looked, and the machine keeps it.
//
// ★The reason it compares elements and not pixels is written in `cmd/still`, which has the pixel
// version and says it cannot be used this way: its clean control reports **6136** differing pixels
// between two frames, a fifth of the picture, because a colour register sweeps in every build.
// Here a static picture reads **0**.
//
// Designed by the mailing-list distillation (helper-3), who found the `diffPixels` trap before
// proposing the measure; implemented and measured here.
func TestMaxFlickerAreaGatesElementsNotColours(t *testing.T) {
	t.Chdir("../..")

	run := func(rom string, max int) (bool, string) {
		t.Helper()
		res, err := Run(&Scenario{Rom: rom, Checks: &Checks{MaxFlickerArea: &max}}, false)
		if err != nil {
			t.Fatal(err)
		}
		var d []string
		for _, a := range res.Asserts {
			d = append(d, a.Desc)
		}
		return res.Pass, strings.Join(d, " | ")
	}

	// ★A static picture must pass a ceiling of ZERO. This is the assertion that would fail if the
	// check ever started counting colour changes, and it is the whole reason the measure is worth
	// having rather than a pixel diff.
	if pass, d := run("roms/litmus/litmus_pal.asm", 0); !pass {
		t.Errorf("a static picture failed a ceiling of zero: %s", d)
	}

	// ★★A multiplexed kernel must NOT pass a ceiling of zero, or the check answers zero for
	// everything and the control above is vacuous.
	pass, d := run("roms/techniques/zone_multiplex.asm", 0)
	if pass {
		t.Errorf("a zone-multiplexed kernel passed a ceiling of zero flickering pixels: %s", d)
	}
	if !strings.Contains(d, "counts ELEMENTS, not colours") {
		t.Errorf("the failure did not explain what was counted: %s", d)
	}

	// ★★★And it must pass a ceiling above its actual area, so the check is a threshold rather
	// than a prohibition.
	if pass, d := run("roms/techniques/zone_multiplex.asm", 200); !pass {
		t.Errorf("a kernel whose flicker area is 126 failed a ceiling of 200: %s", d)
	}
}
