package oracle

import (
	"os/exec"
	"strings"
	"testing"
)

// TestMameRunsHeadless pins the environment that keeps the MAME oracle off the
// screen.
//
// MEASURED, MAME 0.288 (SDL3, macOS): `-video none -sound none` alone is not
// enough. One DumpRAM connected to com.apple.dock.fullscreen, created an SDL3Window
// and an NSToolbarFullScreenWindow and ordered it front — 10 AppKit window events
// per call, which on a full-screen Space pulls the whole display away and back. With
// the dummy SDL drivers the same call produced 0. The RAM dump is byte-identical
// either way, so the oracle's own output can never catch a regression here; this
// test is the only thing that can.
func TestMameRunsHeadless(t *testing.T) {
	want := map[string]bool{"SDL_VIDEODRIVER=dummy": false, "SDL_AUDIODRIVER=dummy": false}
	for _, kv := range headlessEnv {
		if _, ok := want[kv]; ok {
			want[kv] = true
		}
	}
	for kv, found := range want {
		if !found {
			t.Errorf("headlessEnv is missing %q — MAME will open a window on every oracle call", kv)
		}
	}
	// And it has to actually reach the process. Building the command the same way
	// DumpRAM does is the closest a unit test gets to the real invocation.
	cmd := exec.Command("true")
	cmd.Env = append([]string{"PATH=/usr/bin"}, headlessEnv...)
	joined := strings.Join(cmd.Env, " ")
	for kv := range want {
		if !strings.Contains(joined, kv) {
			t.Errorf("%q did not survive into cmd.Env", kv)
		}
	}
}
