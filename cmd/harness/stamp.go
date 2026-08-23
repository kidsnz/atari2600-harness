package main

// Which build answered, and has the source moved since?
//
// A static-analysis tool returns a claim about SOURCE CODE, and the MCP server that
// answers is whatever binary was running when the session connected. Editing the
// analyser does not change it. The result carries no sign of this, so a stale server
// reports the old answer with full confidence and the reader has no way to tell.
//
// Measured 2026-08-01, twice in one session, both times on a fix that was already
// correct:
//
//   - `prove_line_budget` kept returning worst=74 for a kernel the current source
//     proves at 66, because the running binary predated the page-aligned-base change.
//   - the DAG-first witness ROM came back "multiple back-edges — not modeled in v1"
//     while the current source bounded it at 26.
//
// Both read as "the change did not work". Both were caught only by re-running the
// analysis from Go instead of through the tool. Neither would have survived a result
// that said which commit answered.
//
// Go already embeds what is needed, with no build flags: `debug.ReadBuildInfo`
// carries `vcs.revision`, `vcs.time` and `vcs.modified` for any binary built inside a
// git work tree. The running server reports bb3b0f8 while the tree sits four commits
// later, which is the whole story in one line.
//
// This goes further than stamping, because a stamp only helps a reader who thinks to
// compare. The server runs on the same machine as the repository, so it reads HEAD
// itself and says STALE in the result. A tool that requires the reader to notice is
// weaker than one that says what it did not do — the standing rule in this project.

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/kidsnz/atari2600-harness/internal/version"
)

// AnalyzerStamp identifies the build that produced a static-analysis answer.
type AnalyzerStamp struct {
	Version  string `json:"version"`
	Revision string `json:"revision,omitempty"` // short git revision the binary was built from
	Built    string `json:"built,omitempty"`    // commit time of that revision
	Dirty    bool   `json:"dirty,omitempty"`    // the work tree had uncommitted changes at build time
	// Stale is a full sentence rather than a flag, so it cannot be skimmed past. It
	// is set only when the repository's HEAD differs from Revision — that is, when
	// this answer describes source that is no longer what is on disk.
	Stale string `json:"stale,omitempty"`
}

var (
	stampOnce sync.Once
	stampVal  AnalyzerStamp

	// headRevisionFn is a seam, not a convenience. The half of this file that broke was the half
	// no test could reach; making the repository lookup replaceable is what lets a test assert
	// that it happens once PER CALL rather than once per process.
	headRevisionFn = headRevision
)

// analyzerStamp reports the running build.
//
// THE BUILD HALF IS COMPUTED ONCE and the STALE HALF IS NOT, and the split is the point. The
// binary's own identity — version, revision, build time, whether the tree was dirty when it was
// made — is fixed at link time and cannot change while the process runs. HEAD can, and does: this
// is a long-lived MCP server that the author reconnects to rarely.
//
// Both used to be inside the same sync.Once, on the reasoning that "HEAD moving under a live
// server is exactly the case being reported, so re-reading it per call would let the warning
// disappear on its own". That does not hold. The only way a fresh read makes the warning vanish
// is HEAD returning to the revision the binary was built from — at which point the binary really
// is current and silence is the right answer. What the caching actually bought was the opposite
// failure, measured by the other session on 2026-08-23: a server started when nothing was stale
// reported "not stale" FOREVER, however many commits landed afterwards. That is precisely the
// shape this project keeps finding — a check that cannot fail — and it was invisible until the
// commit before this one made the warning able to fire from the umbrella at all.
func analyzerStamp() AnalyzerStamp {
	stampOnce.Do(func() {
		s := AnalyzerStamp{Version: version.Harness}
		if bi, ok := debug.ReadBuildInfo(); ok {
			for _, kv := range bi.Settings {
				switch kv.Key {
				case "vcs.revision":
					s.Revision = shortRev(kv.Value)
				case "vcs.time":
					s.Built = kv.Value
				case "vcs.modified":
					s.Dirty = kv.Value == "true"
				}
			}
		}
		stampVal = s
	})
	out := stampVal
	out.Stale = staleNote(out.Revision, headRevisionFn(), out.Dirty)
	return out
}

func shortRev(r string) string {
	if len(r) > 7 {
		return r[:7]
	}
	return r
}

// staleNote returns the warning to put in a result, or "" when there is nothing to
// warn about. Kept as a pure function of its three inputs so it can be exercised
// without arranging a build: the interesting cases are combinations of values, not
// of filesystems.
func staleNote(built, head string, dirty bool) string {
	if built == "" || head == "" {
		return "" // not built from a work tree, or the repository is not reachable
	}
	if built == head {
		if dirty {
			return "this binary was built from " + built + " with UNCOMMITTED changes in the tree, " +
				"so the source it analysed is not any commit; rebuild to be sure what answered"
		}
		return ""
	}
	return "STALE: this answer came from a binary built at " + built + ", but the repository is now at " +
		head + ". If you changed the analyser, this result predates the change — rebuild bin/harness and " +
		"reconnect, or run the analysis from Go. A correct fix reported through a stale server reads " +
		"exactly like a fix that did not work."
}

// headRevision reads the repository's current commit without shelling out, anchored to the
// BINARY'S OWN LOCATION rather than to the working directory.
//
// WHY NOT THE WORKING DIRECTORY, measured 2026-08-23. `.mcp.json` sets no `cwd`, so the server
// inherits the client's, which is the UMBRELLA directory holding harness/, roms/ and sandbox/.
// The umbrella belongs to no repository ON PURPOSE — that placement is what makes publishing
// reference/ structurally impossible — so the walk reached / without finding a .git and returned
// "". staleNote() opens with `if built == "" || head == ""`, so BOTH warnings it can raise died
// there: the STALE sentence and, in the same branch, the one about a binary built from a dirty
// tree. On that day bin/harness had been built at 845656c while the repository sat at 2817b25,
// and a full day of static-analysis answers came back through it with no `stale` field at all.
// The `dirty` FLAG survived, because it is stamped at build time — so the skimmable form lived
// and the full sentence, written that way precisely so it could not be skimmed past, did not.
//
// WHY THERE IS NO WORKING-DIRECTORY FALLBACK. From roms/ the walk finds roms/.git and would
// compare a harness build revision against a DIFFERENT repository's HEAD, reporting STALE
// forever — the false positive the paragraph above forbids. Anchoring to the executable has no
// such failure: bin/harness lives inside the repository it was built from.
//
// The limits, so the next reader does not re-add the fallback: under `go run` and `go test` the
// binary sits in a temp directory, the walk finds nothing, and the warning stays silent. That is
// the same silence as before this change, and it is the safe direction. A binary built in one
// repository and copied into another's bin/ would report against the wrong one; that does not
// happen in this deployment, and it is why headRevisionFrom takes its directory as an ARGUMENT
// — the case that actually broke is only reachable from a test that can choose the directory.
func headRevision() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	// A symlinked binary would otherwise be looked up beside the link rather than beside the
	// real file, which is a different repository or none.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return headRevisionFrom(filepath.Dir(exe))
}

// headRevisionFrom walks up from dir looking for .git, and returns "" rather than guessing when
// anything is unexpected — a wrong HEAD would produce a false STALE, which is worse than none.
func headRevisionFrom(dir string) string {
	for {
		gitPath := filepath.Join(dir, ".git")
		if fi, err := os.Stat(gitPath); err == nil {
			if fi.IsDir() {
				return readHead(gitPath)
			}
			// A LINKED WORKTREE keeps a FILE here, holding "gitdir: <path to the real one>".
			// Without this branch the walk falls through to the parent directory, finds no
			// repository at all, and the stale-binary warning this whole file exists to raise
			// silently stops being able to fire. Found 2026-08-23 by the pre-push gate that
			// runs in exactly such a worktree — and it matters beyond the gate: a session
			// working from its own worktree would get an MCP server that cannot tell it the
			// analyser is out of date.
			if b, err := os.ReadFile(gitPath); err == nil {
				if line := strings.TrimSpace(string(b)); strings.HasPrefix(line, "gitdir:") {
					return readHead(strings.TrimSpace(strings.TrimPrefix(line, "gitdir:")))
				}
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func readHead(gitDir string) string {
	b, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(b))
	if !strings.HasPrefix(head, "ref: ") {
		return shortRev(head) // detached HEAD holds the revision directly
	}
	ref := strings.TrimPrefix(head, "ref: ")
	if b, err := os.ReadFile(filepath.Join(gitDir, filepath.FromSlash(ref))); err == nil {
		return shortRev(strings.TrimSpace(string(b)))
	}
	// A packed ref: the loose file is absent and the revision lives in packed-refs.
	pb, err := os.ReadFile(filepath.Join(gitDir, "packed-refs"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(pb), "\n") {
		if strings.HasSuffix(line, " "+ref) {
			return shortRev(strings.Fields(line)[0])
		}
	}
	return ""
}
