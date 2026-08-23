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
	"sync"
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
// Without it, the walk could return "" for every input and the whole warning would go
// permanently silent while every unit case above still passed — the exact shape of failure
// this project keeps finding.
//
// It drives headRevisionFrom rather than headRevision because THAT IS THE POINT. The version
// of this test that called headRevision() said, in its own failure message, that the warning
// "can never fire in practice" — and could never print it, because `go test` fixes the working
// directory inside the repository, so the condition it named was structurally false. The walk
// now takes its directory as an argument, which is what makes the broken case reachable at all
// (see TestHeadRevisionIsSilentFromTheUmbrella below).
func TestHeadRevisionReadsThisRepository(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	got := headRevisionFrom(cwd)
	if got == "" {
		t.Fatal("the walk found no repository from the test's own directory, which is inside one")
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

// TestHeadRevisionIsSilentFromTheUmbrella stages the layout that actually broke, and is the
// test the old shape of this file could not express.
//
// `.mcp.json` sets no `cwd`, so the server inherits the client's: the umbrella directory that
// holds harness/, roms/ and sandbox/ and belongs to no repository itself. Measured 2026-08-23,
// a full day of static-analysis answers came back from a binary built at 845656c while the
// repository sat at 2817b25, with no `stale` field on any of them.
//
// The three assertions are one argument: a repository BESIDE you is not a repository ABOVE you,
// so anchoring the walk to the working directory can only be wrong here — silent at the umbrella
// and, worse, confidently wrong from a sibling. That is why headRevision anchors to the binary.
func TestHeadRevisionIsSilentFromTheUmbrella(t *testing.T) {
	umbrella := t.TempDir()
	if headRevisionFrom(umbrella) != "" {
		t.Skipf("%s already sits inside a repository, so the umbrella case cannot be staged here", umbrella)
	}
	stage := func(name, sha string) string {
		t.Helper()
		git := filepath.Join(umbrella, name, ".git")
		if err := os.MkdirAll(filepath.Join(git, "refs", "heads"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(git, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(git, "refs", "heads", "main"), []byte(sha+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		bin := filepath.Join(umbrella, name, "bin")
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatal(err)
		}
		return bin
	}
	harnessBin := stage("harness", "2817b25aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	romsBin := stage("roms", "c834f58bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	if got := headRevisionFrom(harnessBin); got != "2817b25" {
		t.Errorf("from harness/bin the walk read %q, want %q", got, "2817b25")
	}
	// A working-directory fallback would have compared a HARNESS build number against this.
	if got := headRevisionFrom(romsBin); got != "c834f58" {
		t.Errorf("from roms/bin the walk read %q, want %q", got, "c834f58")
	}
	if got := headRevisionFrom(umbrella); got != "" {
		t.Errorf("from the umbrella the walk read %q, want \"\": nothing above it is a repository", got)
	}
}

// TestHeadRevisionAnchorsToTheBinaryNotTheWorkingDirectory is the guard against the fallback
// being restored. Under `go test` the binary sits in a temp directory outside every repository,
// so headRevision finds nothing and the warning stays quiet — the same silence as before the
// anchor changed, and the safe direction, since a false STALE is worse than none. A fallback
// that reached for the working directory would make this test read a revision instead, and from
// roms/ the same fallback would compare a HARNESS build number against roms' HEAD and report
// STALE forever.
//
// The first version of this test only logged, and check_tests caught it in the same run that
// this file's other repairs went green: a test with no failure path is a green tick that means
// nothing. It was written by the same hand that had spent the day naming that shape.
func TestHeadRevisionAnchorsToTheBinaryNotTheWorkingDirectory(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	want := headRevisionFrom(filepath.Dir(exe))
	got := headRevision()
	if got != want {
		t.Fatalf("headRevision read %q but the binary's own directory says %q — the anchor is "+
			"not the executable", got, want)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if fromCwd := headRevisionFrom(cwd); want == "" && got == fromCwd && fromCwd != "" {
		t.Fatalf("the binary is in no repository, yet headRevision returned the working "+
			"directory's revision %q: the cwd fallback is back", got)
	}
}

// TestTheStaleHalfIsNotCached is the negative control for the split between the two halves of
// analyzerStamp. A server here lives for days across many calls, so a stale check computed once
// at startup answers about the moment it was launched and nothing after it — the other session
// measured exactly that: a server started when nothing was stale reported "not stale" through
// every later call, however many commits landed. The build half must stay cached (it cannot
// change) and the repository half must not (it does).
func TestTheStaleHalfIsNotCached(t *testing.T) {
	saved := headRevisionFn
	t.Cleanup(func() { headRevisionFn = saved; stampOnce = sync.Once{}; stampVal = AnalyzerStamp{} })

	stampOnce = sync.Once{}
	stampVal = AnalyzerStamp{}
	calls := 0
	head := "aaaaaaa"
	headRevisionFn = func() string { calls++; return head }

	first := analyzerStamp()
	if calls != 1 {
		t.Fatalf("the repository was read %d times on the first call, want 1", calls)
	}
	// The revision the binary was built from is whatever this test binary carries; what matters is
	// that MOVING HEAD changes the answer on the very next call.
	head = "bbbbbbb"
	second := analyzerStamp()
	if calls != 2 {
		t.Fatalf("the repository was read %d times over two calls, want 2 — HEAD is being cached", calls)
	}
	if first.Version != second.Version || first.Revision != second.Revision ||
		first.Built != second.Built || first.Dirty != second.Dirty {
		t.Errorf("the build half changed between calls: %+v then %+v — it is stamped at link "+
			"time and must be computed once", first, second)
	}
	// With a build revision that is neither of the two fake HEADs, both calls should be stale, and
	// each should name the head it actually read.
	if first.Revision != "" {
		if !strings.Contains(first.Stale, "aaaaaaa") {
			t.Errorf("first call's note does not name the HEAD it read: %q", first.Stale)
		}
		if !strings.Contains(second.Stale, "bbbbbbb") {
			t.Errorf("second call still reports the FIRST head: %q", second.Stale)
		}
	}
}
