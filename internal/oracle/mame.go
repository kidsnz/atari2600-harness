package oracle

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Mame is the MAME oracle (VV-6): a genuinely independent third emulator (unlike
// the Stella oracle, which needs a human keypress). It runs the a2600 driver with a
// lua autoboot script that dumps RAM after N frames, catching corners both
// Gopher2600 and Stella might share.
//
// It is headless, but NOT because of -video none: see the environment set in
// DumpRAM and TestMameRunsHeadless for what that option does and does not do.
type Mame struct{ Exe string } // path to the mame binary; "" = look up "mame" on PATH

// MameAvailable reports whether a MAME binary can be found (for test gating).
func MameAvailable() bool {
	_, err := mameExe(Mame{})
	return err == nil
}

func mameExe(m Mame) (string, error) {
	if m.Exe != "" {
		return m.Exe, nil
	}
	return exec.LookPath("mame")
}

func (Mame) Name() string { return "mame" }

// headlessEnv is what actually keeps MAME off the screen. Kept as a package
// variable so a test can assert it is still applied: the failure it prevents is
// invisible in the oracle's own output (the RAM dump is identical either way), so
// nothing else would notice it coming back.
var headlessEnv = []string{"SDL_VIDEODRIVER=dummy", "SDL_AUDIODRIVER=dummy"}

// luaTemplate dumps RAM $80-$FF to OUTPATH after %d frames, then exits.
const luaTemplate = `local count = 0
local mem = manager.machine.devices[":maincpu"].spaces["program"]
emu.add_machine_frame_notifier(function()
  count = count + 1
  if count == %d then
    local f = io.open(%q, "w")
    for a = 0x80, 0xFF do f:write(string.format("%%02x\n", mem:read_u8(a))) end
    f:close()
    manager.machine:exit()
  end
end)
`

func (m Mame) DumpRAM(romPath string, frames int) (RAMDump, error) {
	var d RAMDump
	exe, err := mameExe(m)
	if err != nil {
		return d, fmt.Errorf("mame not found (brew install mame): %w", err)
	}
	absRom, err := filepath.Abs(romPath)
	if err != nil {
		return d, err
	}
	dir, err := os.MkdirTemp("", "mamecheck")
	if err != nil {
		return d, err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "ram.txt")
	lua := filepath.Join(dir, "dump.lua")
	if err := os.WriteFile(lua, []byte(fmt.Sprintf(luaTemplate, frames, out)), 0o644); err != nil {
		return d, err
	}

	// -seconds_to_run is a safety bound; the lua exits at `frames`. -skip_gameinfo
	// so frame counting starts at game power-on (matching the other oracles).
	secs := frames/60 + 5
	cmd := exec.Command(exe, "a2600",
		"-cart", absRom,
		"-video", "none", "-sound", "none",
		"-skip_gameinfo", "-nothrottle",
		"-autoboot_script", lua, "-autoboot_delay", "0",
		"-seconds_to_run", strconv.Itoa(secs),
	)
	cmd.Dir = dir // keep MAME's cfg/nvram scratch out of the repo
	// -video none is NOT enough to keep this off the screen, which is the opposite of
	// what the doc comment above used to claim. Measured on MAME 0.288 (SDL3, macOS):
	// a single DumpRAM connects to `com.apple.dock.fullscreen`, creates an SDL3Window
	// plus an NSToolbarFullScreenWindow and orders it front — 10 AppKit window events
	// per call. On a machine whose terminal is a full-screen Space that yanks the
	// display to another Space and back, once per oracle call, and this oracle runs
	// from `go test ./...`. -sound none likewise still started MIDIServer.
	//
	// SDL decides whether to touch the display server BEFORE MAME's own options are
	// read, so the fix has to be in the environment: the dummy drivers make SDL
	// initialise no video and no audio backend at all.
	cmd.Env = append(os.Environ(), headlessEnv...)
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		// MAME often exits non-zero on -seconds_to_run; only fail if no dump appeared.
		if _, statErr := os.Stat(out); statErr != nil {
			return d, fmt.Errorf("mame run failed: %v\n%s", err, outBytes)
		}
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		return d, fmt.Errorf("mame produced no RAM dump: %w", err)
	}
	lines := strings.Fields(string(raw))
	if len(lines) < 128 {
		return d, fmt.Errorf("mame dump has %d bytes, want 128", len(lines))
	}
	for i := 0; i < 128; i++ {
		v, err := strconv.ParseUint(lines[i], 16, 8)
		if err != nil {
			return d, fmt.Errorf("mame dump parse byte %d: %w", i, err)
		}
		d[i] = byte(v)
	}
	return d, nil
}
