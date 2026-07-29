package behavmatch

import (
	"strings"
	"testing"
)

// The RAM gate's exclusion bookkeeping is sound — measured over the first six
// built-in scenarios on six ROMs, not one excluded byte actually varied in the
// traces, so nothing was dropped that mattered. What the numbers exposed instead
// is how THIN the comparison can be:
//
//	paddle_demo   0 of 128 bytes compared
//	game_states   1
//	bullets       1
//	litmus_bank   2
//	Outlaw       15
//	Combat       45
//
// VACUOUS only fires at zero, so a gate comparing ONE byte printed
// "first_divergence: none" and read as a pass. The mask's thickness is a property
// of the SCENARIOS, not of the ROM, and a reader deciding what "no divergence" is
// worth needs that in front of them.
//
// Both directions are asserted: a thin gate must warn, and a thick one must not,
// or the warning becomes noise that gets ignored.
func TestThinRAMGateSaysSo(t *testing.T) {
	thin, err := Record("../../roms/techniques/game_states.bin", "NTSC", Library["p0-right"], 0)
	if err != nil {
		t.Skipf("ROM unavailable: %v", err)
	}
	g := GateRAM(thin, thin, LiveMask(thin))
	out := g.String()
	if len(g.Compared) > thinGateBytes {
		t.Skipf("premise changed: %d bytes compared, no longer a thin gate", len(g.Compared))
	}
	if !strings.Contains(out, "THIN") {
		t.Errorf("a gate comparing %d of 128 bytes did not warn; it printed:\n%s", len(g.Compared), out)
	}
	if !g.Pass() {
		t.Error("premise broken: a ROM compared against itself must not diverge")
	}
	// The warning must not replace the number, and the number must not replace the
	// warning. Both have to be there.
	if !strings.Contains(out, "1/128 bytes compared") {
		t.Errorf("the count disappeared from the header:\n%s", out)
	}
}

// A gate with real coverage must stay quiet, or the warning stops meaning anything.
func TestThickRAMGateDoesNotWarn(t *testing.T) {
	const combat = "../../../sandbox/studies/combat/Combat.bin"
	var traces []*Trace
	for _, n := range ScenarioNames()[:6] {
		tr, err := Record(combat, "NTSC", Library[n], 0)
		if err != nil {
			t.Skipf("commercial ROM unavailable: %v", err)
		}
		traces = append(traces, tr)
	}
	g := GateRAM(traces[0], traces[0], LiveMask(traces...))
	if len(g.Compared) <= thinGateBytes {
		t.Skipf("premise changed: Combat's mask is now %d bytes, itself a thin gate", len(g.Compared))
	}
	if strings.Contains(g.String(), "THIN") {
		t.Errorf("a gate comparing %d of 128 bytes warned as thin; the warning would then be noise:\n%s",
			len(g.Compared), g.String())
	}
}
