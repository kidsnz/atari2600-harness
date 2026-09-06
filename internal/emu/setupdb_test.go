package emu

import (
	"io/fs"
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
// ★The surface is much wider than the two mandated roots, and the first version of this test got
// that wrong. `go test` runs each package with the **package directory** as the working directory,
// and `resources.JoinPath` creates the folder just for being asked the path — so every package that
// has ever attached a cartridge has minted its own `.gopher2600`. Measured 2026-09-05 with
// `find . -maxdepth 6 -name .gopher2600 -type d`: **38** of them exist under the umbrella, **32 in
// Go package directories** and 6 in directories a human ran a command from (umbrella root,
// `harness/`, `sandbox/`, two work directories, one build directory). All 38 are empty and
// `~/.gopher2600` does not exist. The earlier note here said "four", which was the number this test
// looked for rather than the number that exists — so this test now WALKS instead of listing.
// The mechanism is live, the data is absent, and the count grows with every new package.
//
// ★★This test asserts the absence, because the alternative is that the same commit quietly gives
// different answers on a different machine — or on this one, after someone runs Gopher2600's GUI
// once. The `.gitignore` files already carry `/.gopher2600/`, which says "do not commit this" and
// says nothing about what it can do. Found by the mailing-list distillation (helper-2), who closed
// the population of engine defaults and then followed `AttachCartridge` into it.
func TestNoSetupDatabaseIsSilentlyPatchingOurROMs(t *testing.T) {
	// ★Walk, do not list. Which database is consulted depends on the working directory, and in a
	// dev build the path is relative, so the candidate set is "every directory anything is ever run
	// from" — which `go test` alone makes as large as the package count. `reference/` is skipped
	// because it is 422 MB of other people's material and holds no Go package; `.git` likewise.
	umbrella := filepath.Join("..", "..", "..")

	var found []string
	var dirs int
	skip := map[string]bool{"reference": true, ".git": true, "node_modules": true, "_archive": true}

	walk := func(root string) {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil //nolint:nilerr // an unreadable directory is not this test's business
			}
			name := d.Name()
			if path != root && skip[name] {
				return fs.SkipDir
			}
			if name != ".gopher2600" {
				return nil
			}
			dirs++
			entries, err := os.ReadDir(path)
			if err != nil {
				return fs.SkipDir
			}
			for _, e := range entries {
				found = append(found, filepath.Join(path, e.Name()))
			}
			return fs.SkipDir
		})
	}

	walk(umbrella)
	// A release build, or a portable layout, would reach for the home directory instead.
	if home, err := os.UserHomeDir(); err == nil {
		if entries, err := os.ReadDir(filepath.Join(home, ".gopher2600")); err == nil {
			for _, e := range entries {
				found = append(found, filepath.Join(home, ".gopher2600", e.Name()))
			}
		}
	}
	sort.Strings(found)

	// A witness: if the walk stops finding the directories at all, the test would pass by looking
	// at nothing. This session has already shipped one guard that measured its own blind spot.
	if dirs == 0 {
		t.Fatal("found no `.gopher2600` directory anywhere under the umbrella — the walk is broken, " +
			"not the tree; a version of this test that finds nothing passes for the wrong reason")
	}
	t.Logf("walked %d `.gopher2600` directories; %d files in them", dirs, len(found))

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
