package emu

import (
	"crypto/sha1"
	"fmt"
	"sort"
	"testing"
)

// revisionFlags are the eight TIA-revision behaviours the engine can model. **All eight are false by
// default**, and nothing in this repository has ever set one — so every measurement here is the
// answer for a TIA with these bugs absent.
//
// They are not hypothetical. `Gopher2600/hardware/tia/revision/bugs.go` names shipped cartridges for
// them: `LostMOTCK` — *"Example ROMs: Cosmic Ark (starfield) and the barber pole test ROM"*;
// `LateColor` — *"Example ROM: QuickStep"*, and *"some TIAs that are on the edge of tolerance can
// also exhibit this when the TIA is embedded in another device, such as an RGB MOD"*; `LatePFx` —
// *"Example ROM: Pesco"*. `RESPxHBLANK`'s comment adds that the effect *"seems to be affected by
// operating temperature"* and cites `199901/msg00089` — **a post in the corpus being distilled here**,
// in which Eckhard Stolberg, unable to reproduce a colleague's result, asks *"what version of the VCS
// are you using?"* and writes *"maybe I need to revise my 5 pixel delay theory again."*
var revisionFlags = []string{
	"LateVDELGRP0", "LateVDELGRP1", "LateRESPx", "EarlyScancounter",
	"LatePFx", "LateColor", "LostMOTCK", "RESPxHBLANK",
}

func setRevision(e *Emu, name string) {
	r := &e.VCS.Env.Prefs.Revision.Live
	switch name {
	case "LateVDELGRP0":
		r.LateVDELGRP0.Store(true)
	case "LateVDELGRP1":
		r.LateVDELGRP1.Store(true)
	case "LateRESPx":
		r.LateRESPx.Store(true)
	case "EarlyScancounter":
		r.EarlyScancounter.Store(true)
	case "LatePFx":
		r.LatePFx.Store(true)
	case "LateColor":
		r.LateColor.Store(true)
	case "LostMOTCK":
		r.LostMOTCK.Store(true)
	case "RESPxHBLANK":
		r.RESPxHBLANK.Store(true)
	}
}

// TestWhichLitmusDependOnTIARevisionDefaults answers, for the whole litmus corpus, the question
// `known-traps.md` already asks about `EmulateSARA`:
//
//	"A litmus written for phantom reads today would come back green because THE FEATURE IS OFF, not
//	 because the hardware behaves — it would be MEASURING OUR OWN DEFAULT."
//
// Measured 2026-09-04 over 145 litmus ROMs, rendering each with all flags off and again with one
// flag on, comparing the whole frame: **ten ROMs change, and each changes under exactly one flag.**
//
//	LateColor  -> litmus_bpl_trip, litmus_dag_region, litmus_deadbranch, litmus_pagealign, litmus_pcm
//	LatePFx    -> litmus_pf0_reflect, pf_late, pf_wraps
//	LostMOTCK  -> litmus_hmove_mid, litmus_hmove_side
//
// So ten measured facts here are statements about a TIA without those bugs. That is not wrong — it
// is a scope that was never written down, and this test writes it down.
//
// **`litmus_respx_phase` is deliberately checked and is NOT in the list**, which matters because the
// +5 / +4 strobe phase it measures is the fact `RESPxHBLANK` looked most likely to disturb. Reading
// `video/player.go` says why: `RESPxHBLANK` only applies when a strobe lands at `hsync == 16 || 18`
// on rising phi2 — the very end of HBLANK — and `LateRESPx` only when a strobe falls inside HBLANK
// while an HMOVE ripple has just started. That ROM strobes in the visible area, so neither condition
// is reachable. **The +5 is safe for the case it measures, and unmeasured for the two cases these
// flags govern** — which is the honest form of the claim.
//
// Five flags move nothing here at all (`LateVDELGRP0`, `LateVDELGRP1`, `LateRESPx`,
// `EarlyScancounter`, `RESPxHBLANK`): the corpus has no fixture that reaches their conditions.
//
// Found by the mailing-list distillation (helper-2), who counted the engine's untouched preferences
// and noticed that one of them governs a fact we had just certified.
func TestWhichLitmusDependOnTIARevisionDefaults(t *testing.T) {
	if testing.Short() {
		t.Skip("renders 145 ROMs twice")
	}
	want := map[string]string{
		"litmus_bpl_trip":    "LateColor",
		"litmus_dag_region":  "LateColor",
		"litmus_deadbranch":  "LateColor",
		"litmus_pagealign":   "LateColor",
		"litmus_pcm":         "LateColor",
		"litmus_pf0_reflect": "LatePFx",
		"pf_late":            "LatePFx",
		"pf_wraps":           "LatePFx",
		"litmus_hmove_mid":   "LostMOTCK",
		"litmus_hmove_side":  "LostMOTCK",
	}
	digest := func(rom, flag string) (string, bool) {
		e, err := New("NTSC")
		if err != nil {
			return "", false
		}
		if flag != "" {
			setRevision(e, flag)
		}
		if err := e.LoadROM("../../roms/litmus/" + rom + ".bin"); err != nil {
			return "", false
		}
		if err := e.RunFrames(6); err != nil {
			return "", false
		}
		h := sha1.New()
		for line := 0; line < 262; line++ {
			runs, _, err := e.ReadRow(line)
			if err != nil {
				continue
			}
			for _, r := range runs {
				fmt.Fprintf(h, "%d:%d:%s,", r.Clock, r.Len, r.Hex)
			}
		}
		return fmt.Sprintf("%x", h.Sum(nil)), true
	}

	for rom, flag := range want {
		base, ok := digest(rom, "")
		if !ok {
			t.Skipf("%s unavailable", rom)
		}
		got, _ := digest(rom, flag)
		if got == base {
			t.Errorf("%s no longer changes under %s. Either the fixture stopped exercising the "+
				"condition, or the engine stopped modelling it — either way the scope note in this "+
				"file is now wrong", rom, flag)
		}
		// and only that flag
		var others []string
		for _, f := range revisionFlags {
			if f == flag {
				continue
			}
			if d, _ := digest(rom, f); d != base {
				others = append(others, f)
			}
		}
		if len(others) > 0 {
			sort.Strings(others)
			t.Errorf("%s now also changes under %v; it used to depend on %s alone", rom, others, flag)
		}
	}

	// The one that matters most: the strobe-phase litmus must NOT depend on any of them, because
	// `fundamentals-audit` states its +5 as a hardware fact.
	base, ok := digest("litmus_respx_phase", "")
	if !ok {
		t.Skip("litmus_respx_phase unavailable")
	}
	for _, f := range revisionFlags {
		if d, _ := digest("litmus_respx_phase", f); d != base {
			t.Errorf("litmus_respx_phase changes under %s — the +5/+4 strobe phase is then a "+
				"statement about a TIA with that bug absent, and `fundamentals-audit` needs to say "+
				"so", f)
		}
	}
}
