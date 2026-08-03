package ceiling

import (
	"math"
	"os"
	"testing"
)

// Commercial cartridges live in the umbrella `reference/` tree, which is NOT part
// of this repo, so CI does not have them. They are used here only as sources of
// pictures — nothing is read from them but the frames they render.
const refRoot = "../../../reference/pizza-boy/Samples for Pizza Boy/"

var commercialFrames = []struct {
	name string
	file string
}{
	{"Chopper Command", "Chopper Command.bin"},
	{"Seaquest", "Seaquest.bin"},
	{"Barnstorming", "Barnstorming.bin"},
	{"Pressure Cooker", "Pressure Cooker.bin"},
	{"Vanguard", "Vanguard.bin"},
}

// The design finding this metric exists to deliver, checked on real pictures:
// C1->C3 (what the 4-clock column grid itself costs) is LARGER on content made of
// fine sprite detail than on content made of landscape. Barnstorming is the fine
// case and Chopper Command the landscape one; the prototype measured 8.95 against
// 3.25 on its own frames.
//
// The skip discipline matters as much as the assertion. A skip that can silently
// become permanent is itself a defect: this test FAILS if the reference tree is
// present but individual ROMs have gone missing, and only skips when the whole
// tree is absent — and either way it says how many frames it graded.
func TestCommercialFramesReproduceTheGridCostOrdering(t *testing.T) {
	pal := ntscPalette(t)

	type row struct {
		name                string
		flat, c1, c2, c3    float64
		gridCost, spriteBuy float64
		proxy               float64
		c1Sum, proxySum     int64 // raw squared error, so a sub-0.001-rmse gap is still visible
	}
	var got []row
	var missing []string
	for _, c := range commercialFrames {
		path := refRoot + c.file
		if _, err := os.Stat(path); err != nil {
			missing = append(missing, c.name)
			continue
		}
		// 60 frames: past the reset/attract warm-up into a drawn picture.
		img := frameOf(t, path, 60)
		a := ladder(t, img, pal)
		c1, _ := a.Result.RMSEOf(C1)
		c2, _ := a.Result.RMSEOf(C2)
		c3, _ := a.Result.RMSEOf(C3)
		proxySum := protoC1(img, a.cols, &pal)
		proxy := math.Sqrt(float64(proxySum) / (float64(a.Result.Pixels) * 3))
		var c1Sum int64
		for _, r := range a.Result.Rungs {
			if r.Rung == C1 {
				c1Sum = int64(r.SumSq)
			}
		}
		got = append(got, row{c.name, a.Result.Flat.RMSE, c1, c2, c3, c1 - c3, c1 - c2, proxy, c1Sum, proxySum})
	}

	if len(got) == 0 {
		if _, err := os.Stat("../../../reference/"); err == nil {
			t.Fatalf("the reference tree exists but none of the %d commercial ROMs were found under %s "+
				"— this test would go on reporting a pass while grading nothing", len(commercialFrames), refRoot)
		}
		t.Skipf("graded 0 of %d commercial frames: the umbrella reference/ tree is not present (expected in CI)", len(commercialFrames))
	}
	if len(missing) > 0 {
		t.Errorf("graded %d of %d commercial frames; missing: %v — a partially-present corpus is a silently shrinking denominator",
			len(got), len(commercialFrames), missing)
	}

	for _, r := range got {
		t.Logf("%-16s flat=%6.2f  C1=%6.2f  C2=%6.2f  C3=%6.2f  |  C1->C2 %5.2f (one sprite)  C1->C3 %5.2f (the grid)  |  cell-mean proxy sum_sq %d vs exhaustive %d (gap %d)",
			r.name, r.flat, r.c1, r.c2, r.c3, r.spriteBuy, r.gridCost, r.proxySum, r.c1Sum, r.proxySum-r.c1Sum)
	}

	var barn, chop *row
	for i := range got {
		switch got[i].name {
		case "Barnstorming":
			barn = &got[i]
		case "Chopper Command":
			chop = &got[i]
		}
	}
	if barn == nil || chop == nil {
		t.Fatalf("graded %d frames but not both of the two the ordering claim is about", len(got))
	}
	if barn.gridCost <= chop.gridCost {
		t.Errorf("the grid should cost MORE on Barnstorming (fine sprite detail, %.2f) than on Chopper Command "+
			"(landscape, %.2f) — the measured design finding does not reproduce", barn.gridCost, chop.gridCost)
	}
	for _, r := range got {
		if r.proxy < r.c1-1e-9 {
			t.Errorf("%s: the cell-mean proxy (%.4f) beat the exhaustive pixel-level optimum (%.4f)", r.name, r.proxy, r.c1)
		}
	}
	t.Logf("graded %d of %d commercial frames; grid cost Barnstorming %.2f > Chopper Command %.2f",
		len(got), len(commercialFrames), barn.gridCost, chop.gridCost)
}
