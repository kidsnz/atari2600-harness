package scenario

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoBinScenarioRequestsASourceCheck guards a trap the umbrella CLAUDE.md names by hand and
// nothing enforced: two checks need the assembly source, and when a scenario's `rom` is a `.bin`
// they do not fail — they record "skipped (needs .asm source)" with Pass: true. A skip is written
// down as a pass, so the scenario stays green while proving nothing.
//
// Ten of our scenarios point at a `.bin`, and every one of them has a `.asm` beside it, so none is
// a `.bin` out of necessity. **None of the ten currently asks for either check**, which is why this
// has cost nothing so far — and is exactly the state in which someone adds `prove_line_budget` to
// one of them and gets a green tick for a proof that never ran.
//
// The gate is therefore on the combination, not on the `.bin`: a scenario may name a `.bin` (smoke
// and fuzz fixtures have no line-budget intent), and it may ask for a source check, but not both.
// Fix by pointing `rom` at the `.asm` — which is what the umbrella has told authors to do since
// Frogger's scenarios read green for months on `.bin` paths.
func TestNoBinScenarioRequestsASourceCheck(t *testing.T) {
	roots := []string{"../../roms"}
	sourceChecks := []string{"prove_line_budget", "pf_deadlines"}

	var offenders []string
	for _, root := range roots {
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || filepath.Ext(p) != ".json" {
				return nil
			}
			if filepath.Base(filepath.Dir(p)) != "scenarios" {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			var s struct {
				Rom    string                     `json:"rom"`
				Checks map[string]json.RawMessage `json:"checks"`
			}
			if json.Unmarshal(b, &s) != nil {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(s.Rom), ".bin") {
				return nil
			}
			for _, c := range sourceChecks {
				if _, asked := s.Checks[c]; asked {
					offenders = append(offenders,
						p+": rom is "+s.Rom+" but checks include "+c)
				}
			}
			return nil
		})
	}

	if len(offenders) > 0 {
		t.Fatalf("scenario asks for a check that needs the .asm source while naming a .bin, so the "+
			"check will record \"skipped\" as a PASS:\n  %s\n\nPoint `rom` at the .asm instead.",
			strings.Join(offenders, "\n  "))
	}
}
