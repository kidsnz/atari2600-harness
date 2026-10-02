package cyclebound

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// TestTwoLineRegionHidesAPerLineOverrun is a negative control for this package's own prover.
//
// `roms/techniques/score6.asm`'s six-store choreography must finish inside ONE scanline — the
// source says so at the branch: `bpl Krow ; 72 (<76 で行内完結)`. `roms/litmus/litmus_store7_overrun.asm`
// is that file with one extra `lda (zp),y` + `sta GRPn`, eight cycles the line does not have.
//
// Measured 2026-09-07:
//
//	                    Krow worst / budget      Prove says       rendered frame
//	score6                93 / 152               certified        262 scanlines
//	litmus_store7_overrun 102 / 152               CERTIFIED        269 scanlines
//
// **The prover certifies a kernel that adds seven scanlines to the frame.** The region begins with
// `sta WSYNC`, so it is treated as spanning two lines and given 152 cycles; an eight-cycle overrun of
// the *inner* line vanishes into that allowance. Nothing here is broken by accident — a two-line
// region is a real thing and 152 is the right budget for one — but the consequence is that
// `prove_line_budget` **cannot be the only check on a loop whose region starts with a WSYNC**. The
// frame-length check (`ntsc_frame_lines`, `frame_lines_stable`) is what catches this, and this test
// exists so that fact is measured rather than assumed.
//
// The seventh store also buys nothing: the rendered band is 46 px wide either way. The "6" in
// "six-store choreography" was never a cycle budget — it is two players times three NUSIZ copies, and
// with each player placed once for the line, as in this kernel, there is no seventh place to put a
// seventh image. That is the answer to the question this litmus
// was built for, raised by the mailing-list distillation (helper-2): *"is there room in the 76 cycles
// for a 7th store?"* There is room, and it does not help.
func TestTwoLineRegionHidesAPerLineOverrun(t *testing.T) {
	krow := func(asm string) (worst, budget int, certified bool) {
		r, err := Prove(asm, 76)
		if err != nil {
			t.Fatalf("%s: %v", asm, err)
		}
		for _, l := range r.Lines {
			if len(l.StartLoc) >= 4 && l.StartLoc[:4] == "Krow" {
				return l.Worst, l.Budget, r.Certified
			}
		}
		t.Fatalf("%s: no Krow region in the report — the kernel was renamed and this control is "+
			"pointing at nothing", asm)
		return
	}

	w6, b6, c6 := krow("../../roms/techniques/score6.asm")
	w7, b7, c7 := krow("../../roms/litmus/litmus_store7_overrun.asm")

	if b6 != 152 || b7 != 152 {
		t.Fatalf("the Krow regions no longer get a two-line budget (%d and %d, want 152 each). "+
			"That allowance is the whole subject of this test", b6, b7)
	}
	if w7 <= w6 {
		t.Errorf("the seven-store kernel measures %d cycles against the six-store kernel's %d — "+
			"the extra load and store are supposed to cost about eight", w7, w6)
	}
	if !c6 {
		t.Error("score6 is no longer certified, so it has stopped being the control this test " +
			"compares against")
	}
	// The point: the prover says yes to a kernel the machine says no to.
	if !c7 {
		t.Error("the prover now REJECTS the seven-store kernel. That is better than it was on " +
			"2026-09-07, when it certified a kernel that renders 269 scanlines — but it means the " +
			"gap this control documents has been closed, and the note in " +
			"litmus_store7_overrun.asm should be rewritten rather than left describing a hole " +
			"that no longer exists")
	}
}

// atLinesPattern finds a source that declares a line count at all; the prover's own reader
// (regionLines in Prove) decides which region each declaration belongs to.
var atLinesPattern = regexp.MustCompile(`@lines\s+\d`)

// linesFrames is how long the machine runs per ROM when an `@lines N` declaration is checked
// against it. Two seconds of play: long enough for every annotated region in the corpus to run,
// short enough that the whole corpus is a few seconds.
const linesFrames = 120

// scenarioDrive is the part of a scenario file this check replays: when the inputs go in. It is
// read here rather than through internal/scenario because that package imports this one.
type scenarioDrive struct {
	WarmupFrames int `json:"warmup_frames"`
	Inputs       []struct {
		Frame   int     `json:"frame"`
		Player  int     `json:"player"`
		Action  string  `json:"action"`
		Pressed bool    `json:"pressed"`
		Value   float64 `json:"value"`
	} `json:"inputs"`
}

// loadDrive reads roms/<dir>/scenarios/<name>.json next to asm, if there is one. A region that
// only runs when a button is pressed (sfx_demo's sound-effect tick) is measured only if the
// machine is driven the way the ROM's own scenario drives it.
func loadDrive(asm string) (scenarioDrive, error) {
	d := scenarioDrive{WarmupFrames: 2}
	name := strings.TrimSuffix(filepath.Base(asm), ".asm") + ".json"
	b, err := os.ReadFile(filepath.Join(filepath.Dir(asm), "scenarios", name))
	if os.IsNotExist(err) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return d, fmt.Errorf("%s: %w", name, err)
	}
	if d.WarmupFrames == 0 {
		d.WarmupFrames = 2 // internal/scenario's default
	}
	return d, nil
}

// machineRegionCycles runs asm's binary for at least `frames` frames (longer if its scenario's
// inputs reach further), applying that scenario's inputs on the frames it names, and returns, for
// every WSYNC store that opened a region, the most CPU cycles that region took on the machine.
//
// A region's length is the sum of the cycles of the instructions executed between the opening
// strobe and the closing one, both included: the engine's cycle counter (emu.TotalCycles) adds an
// instruction's cycles when it completes and adds nothing for the WSYNC halt itself, so the
// difference of the counter at the two strobes is exactly the prover's Region.Worst quantity, "from
// the release of the opening WSYNC to the completion of the closing store". It is counted, not
// derived from beam coordinates, so it does not lose the region that opens on the last VSYNC line
// and closes in the next frame. emu.ProfileLineWorst does lose that one (it drops every
// interval that crosses the engine's frame boundary and counts the drops), and the region it loses
// is precisely the vblank-top region this check is about whenever it spills into a second line.
func machineRegionCycles(asm string, frames int) (map[uint16]int, error) {
	drive, err := loadDrive(asm)
	if err != nil {
		return nil, err
	}
	e, err := emu.New("NTSC")
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(build.BinPathFor(asm)); err != nil {
		return nil, err
	}
	if _, banks := e.CartInfo(); banks > 1 {
		// Keyed by PC alone; two banks executing a WSYNC at the same address would merge.
		return nil, fmt.Errorf("%s is bank-switched; this measurement keys regions by PC only", asm)
	}
	total := frames
	for _, in := range drive.Inputs {
		if n := drive.WarmupFrames + in.Frame + 60; n > total {
			total = n
		}
	}
	worst := map[uint16]int{}
	var (
		open   uint16
		openAt int64
		have   bool
	)
	start := e.Coords().Frame
	for f := 0; f < total; f++ {
		for _, in := range drive.Inputs {
			if drive.WarmupFrames+in.Frame != f {
				continue
			}
			switch in.Action {
			case "paddle":
				err = e.SetPaddle(in.Player, in.Value)
			case "reset", "select", "color", "p0pro", "p1pro":
				err = e.SetPanel(in.Action, in.Pressed)
			default:
				err = e.SetInput(in.Player, in.Action, in.Pressed)
			}
			if err != nil {
				return nil, fmt.Errorf("%s frame %d %s: %w", asm, in.Frame, in.Action, err)
			}
		}
		for e.Coords().Frame < start+f+1 {
			if e.VCS.CPU.Jammed {
				return nil, fmt.Errorf("%s: CPU jammed at frame %d", asm, e.Coords().Frame)
			}
			if err := e.StepInstruction(); err != nil {
				return nil, err
			}
			w, ok := e.LastTIAWrite()
			if !ok || w.Reg != 0x02 {
				continue
			}
			now := e.TotalCycles()
			if have {
				if c := int(now - openAt); c > worst[open] {
					worst[open] = c
				}
			}
			open, openAt, have = w.PC, now, true
		}
	}
	return worst, nil
}

// linesDecl is one region whose budget an `@lines N` (N >= 2) declaration widened, with what the
// prover and the machine say about it.
type linesDecl struct {
	asm      string
	loc      string
	start    uint16
	lines    int  // N, read back from the budget the prover gave the region
	bounded  bool // false: the prover refuses the region whatever N says
	worst    int  // the prover's bound (bounded only)
	measured int  // the machine's worst over linesFrames frames; -1 = the region never ran
}

// needs is how many scanlines the machine's worst run of this region occupies.
func (d linesDecl) needs() int { return (d.measured + DefaultBudget - 1) / DefaultBudget }

// slack reports whether the declaration grants a line the machine never uses. That extra line is
// not harmless headroom: the prover budgets the region at N*76, so a region that should fit one
// line can grow into a second, add a scanline to the frame, and still certify.
func (d linesDecl) slack() bool { return d.bounded && d.measured >= 0 && d.needs() < d.lines }

func (d linesDecl) String() string {
	m := "never ran"
	if d.measured >= 0 {
		m = fmt.Sprintf("machine %d cy = %d line(s)", d.measured, d.needs())
	}
	p := "prover: unbounded"
	if d.bounded {
		p = fmt.Sprintf("prover %d/%d", d.worst, d.lines*DefaultBudget)
	}
	return fmt.Sprintf("%s $%04X (%s): @lines %d; %s; %s", d.asm, d.start, d.loc, d.lines, p, m)
}

// linesDeclarations proves asm, measures it, and returns every region the prover budgeted at more
// than one line.
func linesDeclarations(asm string, frames int) ([]linesDecl, error) {
	rep, err := Prove(asm, DefaultBudget)
	if err != nil {
		return nil, err
	}
	cyc, err := machineRegionCycles(asm, frames)
	if err != nil {
		return nil, err
	}
	var out []linesDecl
	for _, r := range append(append([]Region{}, rep.BlankLines...), rep.Lines...) {
		if r.Budget <= DefaultBudget {
			continue
		}
		d := linesDecl{asm: filepath.Base(asm), loc: r.StartLoc, start: r.Start,
			lines: r.Budget / DefaultBudget, bounded: r.Bounded, worst: r.Worst, measured: -1}
		if c, ok := cyc[r.Start]; ok {
			d.measured = c
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].start < out[j].start })
	return out, nil
}

// knownSlack lists declarations measured slack that this check does not fix, each with why. An
// entry that stops being slack fails the test, so the list cannot outlive the problem it names.
var knownSlack = map[string]string{
	// Not headroom: sfx_demo is ALREADY broken. Its frame gives this region one line (the overscan
	// loop is a fixed 29), the machine takes two while a sound effect plays (worst 145 cycles, with
	// the scenario's own fire presses), and the prover needs three (159) — so @lines 3 certifies a
	// ROM that renders 263-line frames. Measured 2026-10-02 with `go run ./cmd/scenario` on copies of
	// sfx_demo.json: the scenario's inputs with its asserts removed, so the frame_lines_stable
	// window opens at frame 102 while the last effect still plays, give 262x143 263x27 over 170
	// frames; fire held from frame 0 gives 262x19 263x11 over 30; no input gives 262x170. The
	// scenario itself passes only because its window opens after frame 140, when the effects have
	// ended. Not fixed here: the ROM and its declaration belong to sfx_demo.asm's owner.
	"sfx_demo.asm Vis": "263-line frames while an effect plays; machine 145 (2 lines), frame allows 1, declared 3",
}

// TestLinesDeclarationIsNotSlack closes the hole TestTwoLineRegionHidesAPerLineOverrun documents,
// for the case where the second line was never needed at all.
//
// `@lines N` on a region's opening WSYNC tells the prover the region spans N scanlines, and the
// prover then accepts anything up to N*76 cycles there. Nothing checked that N was true. Measured
// 2026-10-02: `hscroll` and `two_line_vdel` each declared `@lines 2` on their vblank-top region
// while the machine runs it in ONE line (76 cycles at most in both) and the frame is 262 lines. The
// 2 was there to absorb an over-estimate — 79 in hscroll, which charges a page-cross cycle to
// three table reads that never cross; 78 in two_line_vdel, a path where both sprites reverse at
// the bottom in the same frame, which that motion never produces. The cost: a copy of hscroll with
// two more cycles on its scroll path renders 263-line frames and the prover still certified it
// (81/152).
//
// So every region the prover bounds under a widened budget is run on the machine, and the
// declaration must be the number of lines the machine's worst run occupies. A region the machine
// never reaches fails too: a declaration nothing can measure is a declaration nothing checked. The
// measurement is cross-checked against the proof in the other direction (machine <= prover), so a
// measurement that over-counted could not pass this test by inflating every region.
//
// Regions the prover cannot bound are listed and not judged: there the declaration does not change
// the verdict, and the prover already refuses them.
func TestLinesDeclarationIsNotSlack(t *testing.T) {
	var asms []string
	for _, dir := range []string{"../../roms/techniques", "../../roms/litmus"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.asm"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if atLinesPattern.Match(src) {
				asms = append(asms, f)
			}
		}
	}
	if len(asms) == 0 {
		t.Fatal("no source in the corpus carries an @lines declaration — this check is pointing at nothing")
	}
	checked := 0
	seenKnown := map[string]bool{}
	var bad, unjudged []string
	for _, asm := range asms {
		decls, err := linesDeclarations(asm, linesFrames)
		if err != nil {
			t.Fatalf("%s: %v", asm, err)
		}
		for _, d := range decls {
			switch {
			case !d.bounded:
				unjudged = append(unjudged, d.String())
			case d.measured < 0:
				bad = append(bad, d.String()+" — declared, but the region never ran, so nothing measured it")
			case d.measured > d.worst:
				bad = append(bad, d.String()+" — the machine exceeds the proof, so the measurement or the prover is wrong")
			case d.slack():
				key := d.asm + " " + strings.SplitN(d.loc, " ", 2)[0]
				if why, ok := knownSlack[key]; ok {
					seenKnown[key] = true
					t.Logf("known slack, not judged here (%s): %s", why, d)
					continue
				}
				bad = append(bad, d.String()+fmt.Sprintf(" — SLACK: the machine needs %d, so @lines %d lets the "+
					"region overrun into a line the frame does not have and still certify", d.needs(), d.lines))
			default:
				checked++
				t.Logf("ok  %s", d)
			}
		}
	}
	for _, u := range unjudged {
		t.Logf("not judged (the prover refuses this region whatever @lines says): %s", u)
	}
	if len(bad) > 0 {
		t.Errorf("%d @lines declaration(s) do not match the machine:\n  %s", len(bad), strings.Join(bad, "\n  "))
	}
	for key, why := range knownSlack {
		if !seenKnown[key] {
			t.Errorf("knownSlack lists %q (%s), but it is no longer measured slack — remove the entry", key, why)
		}
	}
	if checked == 0 {
		t.Error("no @lines declaration was judged true — with nothing passing, a pass of this test would be vacuous")
	}
	t.Logf("%d declaration(s) match the machine across %d source(s); %d not judged", checked, len(asms), len(unjudged))
}

// TestLinesSlackCheckFindsAPaddedDeclaration is the negative control for the check above: put back
// the two-line declaration hscroll carried until 2026-10-02 and the check must call it slack —
// while the prover, given that declaration, certifies. And without it, the prover must refuse a
// copy whose scroll path is two cycles longer (the copy that renders 263-line frames).
func TestLinesSlackCheckFindsAPaddedDeclaration(t *testing.T) {
	src, err := os.ReadFile("../../roms/techniques/hscroll.asm")
	if err != nil {
		t.Fatal(err)
	}
	const decl = "; @lines 1 —"
	if strings.Count(string(src), decl) != 1 {
		t.Fatalf("hscroll.asm no longer carries exactly one %q — this control is pointing at nothing", decl)
	}
	write := func(name, text string) string {
		p := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	padded := write("hscroll.asm", strings.Replace(string(src), decl, "; @lines 2 —", 1))
	rep, err := Prove(padded, DefaultBudget)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Certified {
		t.Errorf("hscroll with a two-line declaration no longer certifies — the hole this control " +
			"reproduces has moved; re-read what it shows")
	}
	decls, err := linesDeclarations(padded, linesFrames)
	if err != nil {
		t.Fatal(err)
	}
	if len(decls) != 1 || !decls[0].slack() {
		t.Fatalf("the padded declaration was not reported slack: %v", decls)
	}
	t.Logf("padded: %s", decls[0])

	const scroll = "        sta phase\nNoScroll:"
	if !strings.Contains(string(src), scroll) {
		t.Fatal("hscroll.asm's scroll path changed shape; this control cannot lengthen it")
	}
	longer := write("hscroll_over.asm", strings.Replace(string(src), scroll,
		"        sta phase\n        nop\nNoScroll:", 1))
	rep, err = Prove(longer, DefaultBudget)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Certified {
		t.Error("the prover certifies hscroll with two more cycles on its scroll path, a copy that " +
			"renders 263-line frames")
	}
}

// TestAnnotationLinesReadTheNextLineOnlyAfterALoneLabel pins where an annotation is looked for.
func TestAnnotationLinesReadTheNextLineOnlyAfterALoneLabel(t *testing.T) {
	src := []string{
		"        sta WSYNC", // 1: an instruction — line 2 is not its annotation
		"        sta WSYNC          ; @lines 2",
		"Krow:", // 3: a lone label — DASM maps the WSYNC below to it
		"        sta WSYNC          ; @lines 3",
		"Kx:     sta WSYNC          ; @lines 4", // 5: label and instruction on one line
		"        ; a comment line",
	}
	for _, c := range []struct {
		ln   int
		want int
	}{{1, 1}, {2, 1}, {3, 2}, {4, 1}, {5, 1}, {6, 1}, {0, 0}, {7, 0}} {
		if got := len(annotationLines(src, c.ln)); got != c.want {
			t.Errorf("line %d: %d line(s) read, want %d", c.ln, got, c.want)
		}
	}
	if got := annotationLines(src, 3); got[1] != src[3] {
		t.Errorf("after a lone label the next line is read; got %q", got)
	}
}
