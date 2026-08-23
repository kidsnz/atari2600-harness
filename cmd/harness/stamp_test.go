package main

// A stale server must say so, and a current one must stay quiet.
//
// The warning exists because a static analysis is a claim about SOURCE, answered by
// whatever binary the session connected to. Twice on 2026-08-01 a correct fix read as
// a failed one for exactly that reason. So the two directions are both asserted: a
// mismatch has to produce a sentence a reader cannot skim past, and a match must
// produce nothing at all — a warning that fires on a current build is noise, and
// noise gets ignored, which is how the real one would be missed.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaleNoteFiresOnlyWhenTheSourceHasMoved(t *testing.T) {
	for _, c := range []struct {
		name         string
		built, head  string
		dirty        bool
		wantStale    bool
		wantMentions []string
		wantSilent   bool
	}{
		{
			name:  "the case that happened: binary four commits behind",
			built: "bb3b0f8", head: "30b492d",
			wantStale: true, wantMentions: []string{"STALE", "bb3b0f8", "30b492d", "rebuild"},
		},
		{
			name:  "current build, clean tree: nothing to say",
			built: "30b492d", head: "30b492d",
			wantSilent: true,
		},
		{
			name:  "current revision but built dirty: the source it read is no commit",
			built: "30b492d", head: "30b492d", dirty: true,
			wantMentions: []string{"UNCOMMITTED", "30b492d"},
		},
		{
			// A binary built outside a work tree, or a repository we cannot read.
			// Guessing here would produce a false STALE, which trains a reader to
			// ignore the real one.
			name:  "no build revision: silent rather than wrong",
			built: "", head: "30b492d",
			wantSilent: true,
		},
		{
			name:  "no HEAD: silent rather than wrong",
			built: "30b492d", head: "",
			wantSilent: true,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := staleNote(c.built, c.head, c.dirty)
			if c.wantSilent {
				if got != "" {
					t.Fatalf("expected silence, got %q", got)
				}
				return
			}
			if got == "" {
				t.Fatalf("expected a warning, got silence")
			}
			if c.wantStale && !strings.Contains(got, "STALE") {
				t.Errorf("a stale binary's warning must be findable by the word STALE: %q", got)
			}
			for _, want := range c.wantMentions {
				if !strings.Contains(got, want) {
					t.Errorf("warning does not mention %q: %q", want, got)
				}
			}
		})
	}
}

// TestHeadRevisionReadsThisRepository checks the half that touches the filesystem.
// Without it, headRevision could return "" for every input and the whole warning
// would go permanently silent while every unit case above still passed — the exact
// shape of failure this project keeps finding.
func TestHeadRevisionReadsThisRepository(t *testing.T) {
	got := headRevision()
	if got == "" {
		t.Fatal("headRevision found no repository from the test's working directory, so the stale " +
			"warning can never fire in practice however well staleNote behaves")
	}
	if len(got) != 7 {
		t.Errorf("expected a 7-character short revision, got %q", got)
	}
	for _, r := range got {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Fatalf("revision %q is not hexadecimal; something other than a commit was read", got)
		}
	}

	// Cross-check against the file the reader would look at, so a plausible-looking
	// wrong answer is caught rather than accepted for its shape.
	repo := findGitDir(t)
	head, err := os.ReadFile(filepath.Join(repo, "HEAD"))
	if err != nil {
		t.Fatalf("read HEAD: %v", err)
	}
	ref := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(head)), "ref: "))
	if b, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(ref))); err == nil {
		if want := strings.TrimSpace(string(b))[:7]; got != want {
			t.Errorf("headRevision returned %q, the repository says %q", got, want)
		}
	}
	t.Logf("HEAD %s", got)
}

func findGitDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		p := filepath.Join(dir, ".git")
		if fi, err := os.Stat(p); err == nil {
			if fi.IsDir() {
				return p
			}
			// A linked worktree keeps a FILE here pointing at the real git directory. The
			// cross-check has to follow it for the same reason headRevision does, or this test
			// fails in exactly the place the pre-push gate runs — and a test that cannot run
			// where the gate runs stops guarding it.
			if b, err := os.ReadFile(p); err == nil {
				if line := strings.TrimSpace(string(b)); strings.HasPrefix(line, "gitdir:") {
					return strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no .git found")
		}
		dir = parent
	}
}
