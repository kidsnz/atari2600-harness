package emu

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestNoSetupDatabaseIsSilentlyPatchingOurROMs guards a reproducibility hole nobody had named.
//
// `internal/emu` attaches every cartridge through `setup.AttachCartridge`, and the engine's own
// `setup/doc.go` says what that package is for:
//
//	Package setup is used to preset the emulation depending on the attached cartridge. ...
//	Toggling of panel switches / Apply patches to cartridge / Television specification
//	... <DB Key>, patch, <SHA-1 Hash>, <patch file>, <notes>
//
// So a database entry keyed by a ROM's SHA-1 can flip the console's switches, **rewrite the
// cartridge's bytes**, or change the TV standard — and `setup.go` says it will *"silently ignore
// absence of setup database"*. Absence is the only reason this has never mattered.
//
// ★And the path is worse than a single well-known location. `resources/dev_path.go` is
// `//go:build !release` and returns the **relative** string `.gopher2600`; this repository builds
// without the `release` tag. So the database that gets consulted depends on the directory the
// command was run from — and `harness/CLAUDE.md` requires exactly that variation: *"Run commands
// from each repo's root (harness's own from `harness/`, ROMs from `roms/`)."* **Two mandated
// working directories mean two possible databases.**
//
// Measured 2026-09-05: four `.gopher2600` directories exist under the umbrella (umbrella root,
// `roms/`, `harness/`, `sandbox/`) and **all four are empty**; `~/.gopher2600` does not exist.
// `resources.JoinPath` creates the folder just for being asked the path, which is why they are
// there at all. The mechanism is live and the data is absent.
//
// ★★This test asserts the absence, because the alternative is that the same commit quietly gives
// different answers on a different machine — or on this one, after someone runs Gopher2600's GUI
// once. The `.gitignore` files already carry `/.gopher2600/`, which says "do not commit this" and
// says nothing about what it can do. Found by the mailing-list distillation (helper-2), who closed
// the population of engine defaults and then followed `AttachCartridge` into it.
func TestNoSetupDatabaseIsSilentlyPatchingOurROMs(t *testing.T) {
	// Every directory a harness command is expected to run from, per CLAUDE.md, plus the home
	// directory in case a release build or a portable layout ever reaches for it.
	roots := []string{"..", "../..", "../../roms", "../../sandbox"}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, home)
	}

	var found []string
	seen := map[string]bool{}
	for _, root := range roots {
		dir := filepath.Join(root, ".gopher2600")
		abs, err := filepath.Abs(dir)
		if err != nil || seen[abs] {
			continue
		}
		seen[abs] = true
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // absent is the state this test wants
		}
		for _, e := range entries {
			found = append(found, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(found)

	if len(found) > 0 {
		t.Errorf("a Gopher2600 resource directory is no longer empty:\n  %s\n\n"+
			"That matters because `setup.AttachCartridge` consults a setup database keyed by each "+
			"ROM's SHA-1, and an entry there can toggle the panel switches, PATCH THE CARTRIDGE "+
			"BYTES, or change the TV specification — with the absence of the database silently "+
			"ignored. In a dev build the path is RELATIVE, so which database is consulted depends "+
			"on the directory the command ran from, and CLAUDE.md requires running from more than "+
			"one. If this content is deliberate, say so here with what it does; if it appeared by "+
			"itself (running the Gopher2600 GUI once is enough), delete it — otherwise this "+
			"repository's measurements stop being reproducible on another machine",
			strings.Join(found, "\n  "))
	}
}
