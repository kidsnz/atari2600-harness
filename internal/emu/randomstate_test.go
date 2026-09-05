package emu

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// randomStateDependent names every ROM in this repository whose RAM after twelve frames CHANGES
// when the engine's `RandomState` preference is turned on. It is a witness, not a policy: the list
// says which regressions are resting on a fixed power-on state.
//
// ★Why this needed measuring. `Gopher2600/hardware/preferences/preferences.go` sets
// `RandomState.Set(false)` as the default, and nothing in this repository overrides it (grep:
// zero hits outside tests). With it off, INTIM starts at 0, all 128 RAM bytes start at 0, and the
// CPU status register starts at 0. **Every scenario here therefore runs from the same power-on
// state every time.** That is the right default for regression — but it means a ROM that reads
// uninitialised memory produces a stable answer, and a scenario that pins that answer is pinning
// the emulator's convention as much as the ROM's behaviour, without saying so anywhere.
//
// Measured 2026-09-05 across 189 ROMs: **eleven** differ. Four of the five carts (RAM-bearing
// mappers), the superchip litmus, the two uninitialised-read witnesses — which are supposed to
// depend on it — plus `litmus_cycles` (whose HM registers still hold the power-on nibble),
// `litmus_bound_proxy` and `litmus_timerwrap_nearmiss`.
//
// Origin: helper-1 asked whether our scenarios are "passing because the randomness is fixed".
// They are, for these eleven, and now the list is a test rather than an assumption.
var randomStateDependent = map[string]bool{
	"cart_3e":                   true,
	"cart_3eplus":               true,
	"cart_dpc":                  true,
	"cart_f4sc":                 true,
	"cart_f6sc":                 true,
	"litmus_bound_proxy":        true,
	"litmus_cycles":             true,
	"litmus_superchip":          true,
	"litmus_timerwrap_nearmiss": true,
	"litmus_uninit_read":        true,
	"uninit_trap":               true,
}

func TestWhichROMsRestOnTheFixedPowerOnState(t *testing.T) {
	if testing.Short() {
		t.Skip("sweeps the whole ROM corpus twice")
	}
	var roms []string
	err := filepath.Walk("../../roms", func(p string, d os.FileInfo, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".bin") {
			roms = append(roms, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(roms)
	if len(roms) < 100 {
		t.Fatalf("found only %d ROMs — the corpus is not where this test thinks it is, and a sweep "+
			"over nothing agrees with any list", len(roms))
	}

	run := func(rom string, rnd bool) ([RAMSize]uint8, error) {
		e, err := New("NTSC")
		if err != nil {
			return [RAMSize]uint8{}, err
		}
		e.VCS.Env.Prefs.RandomState.Set(rnd)
		if err := e.LoadROM(rom); err != nil {
			return [RAMSize]uint8{}, err
		}
		if err := e.RunFrames(12); err != nil {
			return [RAMSize]uint8{}, err
		}
		return e.CurrentRAM()
	}

	got := map[string]bool{}
	swept := 0
	for _, rom := range roms {
		a, err1 := run(rom, false)
		b, err2 := run(rom, true)
		if err1 != nil || err2 != nil {
			continue
		}
		swept++
		if a != b {
			got[strings.TrimSuffix(filepath.Base(rom), ".bin")] = true
		}
	}

	for name := range got {
		if !randomStateDependent[name] {
			t.Errorf("%s now depends on the power-on state and is not in the witness list. Either it "+
				"grew a read of uninitialised memory, or a scenario pinning it is about to start "+
				"pinning the emulator's convention rather than the ROM's behaviour. Add it with a "+
				"reason, or find the read", name)
		}
	}
	for name := range randomStateDependent {
		if !got[name] {
			t.Errorf("%s no longer depends on the power-on state. That may be a fix, but the list "+
				"claims it does — and a witness list that has drifted is worse than none, because "+
				"the next reader trusts it", name)
		}
	}
	t.Logf("swept %d ROMs twice (RandomState off/on): %d depend on the power-on state", swept, len(got))
}
