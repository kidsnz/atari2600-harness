package cyclebound

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kidsnz/atari2600-harness/internal/build"
)

// ProverVersion identifies the proof method; bump on any change to the prover's
// soundness-relevant logic so a certificate is tied to a specific prover.
const ProverVersion = "cyclebound/3 (VV-2 abstract-interp WCET + @lines + cross-bank flow)"

// LineDecl is a `@lines N` declaration the certificate relies on.
type LineDecl struct {
	Line  int `json:"line"`
	Lines int `json:"lines"`
}

// Provenance records everything needed to reproduce and trust a certificate.
type Provenance struct {
	ProverVersion string `json:"prover_version"`
	GopherPin     string `json:"gopher2600_pin,omitempty"`
	DasmVersion   string `json:"dasm_version,omitempty"`
	AsmSHA256     string `json:"asm_sha256"`
	BinSHA256     string `json:"bin_sha256"`
	GeneratedAt   string `json:"generated_at"`
}

// Certificate is a reproducible, attestable record of a kernel's proven
// per-scanline cycle budget (VV-14). Everything but Provenance.GeneratedAt is
// deterministic in the ROM.
type Certificate struct {
	Asm        string     `json:"asm"`
	Budget     int        `json:"budget_cycles_per_line"`
	Certified  bool       `json:"certified"`
	Regions    int        `json:"regions"`
	Blank      int        `json:"blank_regions"`
	MaxWorst   int        `json:"max_worst_cycles"`
	LineDecls  []LineDecl `json:"line_declarations"`
	Violations []Region   `json:"violations,omitempty"`
	Unbounded  []Region   `json:"unbounded,omitempty"`
	Provenance Provenance `json:"provenance"`
}

func sha256file(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h), nil
}

// best-effort provenance probes: never fail a certificate because a probe is
// unavailable (they record the toolchain, they don't gate the proof).
func gopherPin() string {
	out, err := exec.Command("git", "-C", "Gopher2600", "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func dasmVersion() string {
	out, _ := exec.Command("dasm").CombinedOutput()
	for _, ln := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(ln, "DASM") {
			return strings.TrimSpace(ln)
		}
	}
	return ""
}

var certAtLinesRe = regexp.MustCompile(`@lines\s+(\d+)`)

func scanLineDecls(asmPath string) []LineDecl {
	b, err := os.ReadFile(asmPath)
	if err != nil {
		return nil
	}
	var out []LineDecl
	for i, ln := range strings.Split(string(b), "\n") {
		if g := certAtLinesRe.FindStringSubmatch(ln); g != nil {
			n, _ := strconv.Atoi(g[1])
			out = append(out, LineDecl{Line: i + 1, Lines: n})
		}
	}
	return out
}

// Certify proves asmPath at the given budget and returns a citable certificate
// (the prover report plus the @lines lemmas it relies on and full toolchain
// provenance). budget<=0 defaults to one NTSC scanline.
func Certify(asmPath string, budget int) (*Certificate, error) {
	rep, err := Prove(asmPath, budget)
	if err != nil {
		return nil, err
	}
	asmHash, err := sha256file(asmPath)
	if err != nil {
		return nil, fmt.Errorf("hash asm: %w", err)
	}
	binHash, err := sha256file(build.BinPathFor(asmPath))
	if err != nil {
		return nil, fmt.Errorf("hash bin: %w", err)
	}
	return &Certificate{
		Asm:        rep.Asm,
		Budget:     rep.Budget,
		Certified:  rep.Certified,
		Regions:    rep.Regions,
		Blank:      rep.Blank,
		MaxWorst:   rep.MaxWorst,
		LineDecls:  scanLineDecls(asmPath),
		Violations: rep.Violations,
		Unbounded:  rep.Unbounded,
		Provenance: Provenance{
			ProverVersion: ProverVersion,
			GopherPin:     gopherPin(),
			DasmVersion:   dasmVersion(),
			AsmSHA256:     asmHash,
			BinSHA256:     binHash,
			GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}
