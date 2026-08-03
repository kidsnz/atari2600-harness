package oracle

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// --- G4: cross-check the WRITE-ONLY TIA registers against Stella --------------
//
// RAM and pixels were already cross-checked (see docs/stella-oracle.md), but the
// write-only registers were not, and pixels do not settle them: an object whose
// graphics are zero draws the same picture whatever its NUSIZ, so a wrong reading
// of read_tia_registers can hide behind a right picture forever.
//
// Stella reports these registers only through the debugger's TIA tab, i.e. the
// `tia` command, whose output goes to the prompt widget. Measured 2026-08-03 on
// Stella 7.0: `dump 00 3f 1` does NOT reach them — it returns the TIA READ ports
// (collisions/INPT) mirrored every $10 — and autoexec.script cannot carry them
// out either, because Debugger::exec() keeps only the "Executed N commands"
// summary and discards each command's output (a `saveSes` inside autoexec writes
// a 0-byte file). So the text below is captured by typing `tia` then `saveSes` at
// the prompt; see CaptureStellaTIA.
//
// What each field of that text MEANS was fixed by internal/oracle/testdata/
// tiaprobe.asm, which writes one distinct constant to every register and then
// stops touching TIA. Reading the conventions off Gopher2600 instead would have
// made this comparison circular. Established that way:
//   - HM=$7 is the RAW HMxx nibble as written ($70 -> $7), not a signed motion.
//   - a missile/ball size=#N is the RAW 2-bit field, not a pixel width.
//   - GR=%... and the ball's ENABLED are the NEW register, not the VDEL-selected
//     one (probe has GRP0 new=$A5 old=$22 with VDELP0=1, and Stella prints $A5).
//   - PF0 is printed already shifted down ($B0 -> $0b); this package compares the
//     four meaningful bits.
//   - UPPERCASE spells a set flag, lowercase a clear one.

// TIARegs is one reading of the write-only TIA registers, keyed by the canonical
// names in TIARegNames. Every value is an integer so a diff is numeric (iron rule 1).
type TIARegs map[string]int

// TIARegNames is the canonical, ordered set of registers this oracle compares.
// It is exactly the set Stella's `tia` command reports AND emu can recover; the
// ones Stella does not report are listed in TIARegsNotReported.
var TIARegNames = []string{
	"COLUP0", "COLUP1", "COLUPF", "COLUBK",
	"GRP0", "GRP1",
	"NUSIZ0_PLAYER", "NUSIZ1_PLAYER",
	"NUSIZ0_MSIZE", "NUSIZ1_MSIZE",
	"CTRLPF_REFLECT", "CTRLPF_SCORE", "CTRLPF_PRIORITY", "CTRLPF_BLSIZE",
	"REFP0", "REFP1",
	"VDELP0", "VDELP1", "VDELBL",
	"ENAM0", "ENAM1", "ENABL",
	"RESMP0",
	"PF0", "PF1", "PF2",
	"HMP0", "HMP1", "HMM0", "HMM1", "HMBL",
	"AUDC0", "AUDF0", "AUDV0", "AUDC1", "AUDF1", "AUDV1",
}

// TIARegsNotReported names the write-only TIA registers this oracle canNOT
// cross-check, and why. Naming them is the point: a comparison that quietly
// covered a subset would be the vacuous check this project keeps finding.
var TIARegsNotReported = map[string]string{
	"VBLANK": "Stella's `tia` prints only the D1 blanking flag (as VBLANK/vblank), " +
		"not D6/D7 (INPT latch / dump-to-ground), and emu.TIARegisters does not expose VBLANK at all",
	"VSYNC":  "printed only as a flag, and it is a strobe-like state rather than a stored value",
	"NUSIZ_RAW": "neither side reports the raw NUSIZ byte: Stella prints the decoded player mode and " +
		"the missile size field, so bit 3 and bits 6-7 (unused by the TIA) are not compared",
	"CTRLPF_RAW": "same shape as NUSIZ: reflect/score/priority/ball-size are compared, " +
		"the unused bits 3, 6 and 7 are not",
	"GRP0_OLD/GRP1_OLD/ENABL_OLD": "Stella's text output prints only the NEW copy of the " +
		"VDEL-shadowed registers (established by testdata/tiaprobe.asm), so the old copies " +
		"cannot be cross-checked even though emu exposes GfxDataOld",
	"RESMP1": "Stella 7.0 REPORTS it and reports it WRONG: the M1 line's reset flag tracks " +
		"RESMP0, not RESMP1. Measured on two ROMs — tiaprobe.asm writes RESMP0=$02/RESMP1=$00 " +
		"and Stella prints RESET on both missile lines; tiaprobe2.asm writes RESMP0=$00/" +
		"RESMP1=$02 and Stella prints reset on both. The M1 flag equals RESMP0 in both cases " +
		"and equals RESMP1 in neither, so it is not a usable oracle for RESMP1. " +
		"TestStella70MisreportsRESMP1 locks that behaviour so a Stella fix is noticed",
	"strobes (WSYNC/RSYNC/RESP0/RESP1/RESM0/RESM1/RESBL/HMOVE/HMCLR/CXCLR)": "write-only strobes hold no value",
}

// Diff is one disagreement between two readings.
type TIARegDiff struct {
	Reg    string
	Ours   int
	Stella int
}

func (d TIARegDiff) String() string {
	return fmt.Sprintf("%s: harness=%d ($%02x) stella=%d ($%02x)", d.Reg, d.Ours, d.Ours, d.Stella, d.Stella)
}

// DiffTIARegs compares two readings over TIARegNames and returns the
// disagreements plus the number of registers actually compared. A register
// missing from either side is NOT silently skipped: it is returned in `missing`,
// because "compared 0 of 38 and found 0 diffs" must never read as a pass.
func DiffTIARegs(ours, stella TIARegs) (diffs []TIARegDiff, compared int, missing []string) {
	for _, name := range TIARegNames {
		o, okO := ours[name]
		s, okS := stella[name]
		if !okO || !okS {
			missing = append(missing, name)
			continue
		}
		compared++
		if o != s {
			diffs = append(diffs, TIARegDiff{name, o, s})
		}
	}
	return diffs, compared, missing
}

// --- separating a real divergence from the two honest reasons to differ -------

// TIADiffClass says why two correct emulators can report different values for
// the same write-only register. Nothing is swallowed: every classified diff is
// returned with its evidence, exactly as ClassifyDiff does for RAM.
type TIADiffClass string

const (
	// TIADiffReal: neither excuse applies. One of the two is wrong.
	TIADiffReal TIADiffClass = "divergence"
	// TIADiffPhase: "run N frames" does not name a moment. Our side holds
	// Stella's value at some scanline inside the frame, so the two implementations
	// sampled different instants of a register the kernel rewrites every frame.
	TIADiffPhase TIADiffClass = "sub-frame phase"
	// TIADiffPowerOn: the ROM never moves the register off its power-on value and
	// the two emulators picked different power-on values for something a real TIA
	// leaves undefined.
	TIADiffPowerOn TIADiffClass = "undefined at power-on"
)

// ClassifiedTIADiff is one disagreement plus the measurement that explains it.
type ClassifiedTIADiff struct {
	TIARegDiff
	Class    TIADiffClass
	Evidence string
}

func (c ClassifiedTIADiff) String() string {
	return fmt.Sprintf("%s [%s: %s]", c.TIARegDiff.String(), c.Class, c.Evidence)
}

// gopherPowerOnHM is the raw HMxx nibble Gopher2600 reports before any write.
// It keeps the register as (value^$80)>>4, so its zero-valued field reads back as
// nibble 8, i.e. HMxx=$80; Stella's shadow register powers on at $00. A real
// TIA's motion registers are undefined at power-up, so for a ROM that never
// writes them neither number is the right one. Measured on roms/litmus/
// litmus_cycles.bin and uninit_trap.bin, neither of which contains a single
// HMxx or HMCLR write, where all five registers differ by exactly this.
const gopherPowerOnHM = 8

// sampleScanlines is how far past the compared frame ClassifyTIADiffs looks for
// Stella's value. One NTSC frame is 262 lines; 300 covers a whole steady-state
// frame from any starting phase.
const sampleScanlines = 300

func isHMReg(name string) bool { return strings.HasPrefix(name, "HM") }

// ClassifyTIADiffs re-runs the ROM and samples every register at every scanline
// of the following frame, then labels each disagreement from what it measures.
func ClassifyTIADiffs(romPath string, frames int, diffs []TIARegDiff) ([]ClassifiedTIADiff, error) {
	if len(diffs) == 0 {
		return nil, nil
	}
	e, err := emu.New("NTSC")
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(romPath); err != nil {
		return nil, err
	}
	if err := e.RunFrames(frames); err != nil {
		return nil, err
	}
	seen := map[string]map[int]bool{}
	for _, n := range TIARegNames {
		seen[n] = map[int]bool{}
	}
	for i := 0; i < sampleScanlines; i++ {
		regs := gopherTIARegsFrom(e)
		for _, n := range TIARegNames {
			seen[n][regs[n]] = true
		}
		if err := e.StepScanline(); err != nil {
			return nil, err
		}
	}

	out := make([]ClassifiedTIADiff, 0, len(diffs))
	for _, d := range diffs {
		c := ClassifiedTIADiff{TIARegDiff: d, Class: TIADiffReal}
		switch {
		case seen[d.Reg][d.Stella]:
			c.Class = TIADiffPhase
			c.Evidence = fmt.Sprintf("our side also holds $%02x at some scanline of the next frame "+
				"(%d distinct values in %d scanlines)", d.Stella, len(seen[d.Reg]), sampleScanlines)
		case isHMReg(d.Reg) && d.Ours == gopherPowerOnHM && d.Stella == 0 && len(seen[d.Reg]) == 1:
			c.Class = TIADiffPowerOn
			c.Evidence = fmt.Sprintf("%s never changes in %d scanlines and still reads Gopher2600's "+
				"power-on nibble %d; Stella powers the same register on at 0 and the TIA leaves it undefined",
				d.Reg, sampleScanlines, gopherPowerOnHM)
		default:
			c.Evidence = fmt.Sprintf("our side never holds $%02x anywhere in the next %d scanlines "+
				"(%d distinct values seen)", d.Stella, sampleScanlines, len(seen[d.Reg]))
		}
		out = append(out, c)
	}
	return out, nil
}

// --- our side: recover the registers from Gopher2600 --------------------------

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// GopherTIARegs runs the ROM from power-on for `frames` frames on the embedded
// Gopher2600 and reports the write-only registers in the canonical form.
//
// The HM registers are not part of emu.TIARegisters, so they are read from the
// sprite state directly. Gopher2600 stores HMxx as (value^$80)>>4, i.e. the raw
// nibble XOR 8 (see video/ball.go setHmoveValue), so the raw nibble is Hmove^8.
func GopherTIARegs(romPath string, frames int) (TIARegs, error) {
	e, err := emu.New("NTSC")
	if err != nil {
		return nil, err
	}
	if err := e.LoadROM(romPath); err != nil {
		return nil, err
	}
	if err := e.RunFrames(frames); err != nil {
		return nil, err
	}
	return gopherTIARegsFrom(e), nil
}

func gopherTIARegsFrom(e *emu.Emu) TIARegs {
	r := e.ReadTIARegisters()
	a := e.ReadAudio()
	v := e.VCS.TIA.Video
	return TIARegs{
		"COLUP0": int(r.Player0.Color),
		"COLUP1": int(r.Player1.Color),
		"COLUPF": int(r.Playfield.ForegroundColor),
		"COLUBK": int(r.Playfield.BackgroundColor),

		"GRP0": int(r.Player0.GfxNew),
		"GRP1": int(r.Player1.GfxNew),

		"NUSIZ0_PLAYER": int(r.Player0.SizeAndCopies),
		"NUSIZ1_PLAYER": int(r.Player1.SizeAndCopies),
		"NUSIZ0_MSIZE":  int(r.Missile0.Size),
		"NUSIZ1_MSIZE":  int(r.Missile1.Size),

		"CTRLPF_REFLECT":  b2i(r.Playfield.Reflected),
		"CTRLPF_SCORE":    b2i(r.Playfield.Scoremode),
		"CTRLPF_PRIORITY": b2i(r.Playfield.Priority),
		"CTRLPF_BLSIZE":   int(r.Ball.Size),

		"REFP0": b2i(r.Player0.Reflected),
		"REFP1": b2i(r.Player1.Reflected),

		"VDELP0": b2i(r.Player0.VerticalDelay),
		"VDELP1": b2i(r.Player1.VerticalDelay),
		"VDELBL": b2i(r.Ball.VerticalDelay),

		"ENAM0": b2i(r.Missile0.Enabled),
		"ENAM1": b2i(r.Missile1.Enabled),
		"ENABL": b2i(r.Ball.Enabled),

		"RESMP0": b2i(r.Missile0.ResetToPlayer),
		"RESMP1": b2i(r.Missile1.ResetToPlayer),

		// Stella prints PF0 already shifted down; compare the four bits that exist.
		"PF0": int(r.Playfield.PF0) >> 4,
		"PF1": int(r.Playfield.PF1),
		"PF2": int(r.Playfield.PF2),

		"HMP0": int(v.Player0.Hmove) ^ 8,
		"HMP1": int(v.Player1.Hmove) ^ 8,
		"HMM0": int(v.Missile0.Hmove) ^ 8,
		"HMM1": int(v.Missile1.Hmove) ^ 8,
		"HMBL": int(v.Ball.Hmove) ^ 8,

		"AUDC0": int(a.Channel0.Control),
		"AUDF0": int(a.Channel0.Freq),
		"AUDV0": int(a.Channel0.Volume),
		"AUDC1": int(a.Channel1.Control),
		"AUDF1": int(a.Channel1.Freq),
		"AUDV1": int(a.Channel1.Volume),
	}
}

// --- Stella's side: parse the `tia` command's text ----------------------------

// nusizPlayerLabels maps Stella's decoded player NUSIZ label to the 3-bit field.
// Order taken from Stella 7.0's own label table; indices 5 and 6 are additionally
// confirmed at the register by testdata/tiaprobe.asm (NUSIZ0=$16, NUSIZ1=$25).
var nusizPlayerLabels = []string{
	"1 copy",
	"2 copies - close (8)",
	"2 copies - med (24)",
	"3 copies - close (8)",
	"2 copies - wide (56)",
	"2x (16) sized player",
	"3 copies - med (24)",
	"4x (32) sized player",
}

var (
	reColu   = regexp.MustCompile(`P0=\$([0-9a-fA-F]{2}).*?P1=\$([0-9a-fA-F]{2}).*?PF=\$([0-9a-fA-F]{2}).*?BK=\$([0-9a-fA-F]{2})`)
	rePlayer = regexp.MustCompile(`^P([01]): GR=%([01]{8}) +pos=#(-?\d+) +HM=\$([0-9a-fA-F])`)
	reMissil = regexp.MustCompile(`^M([01]): (ENABLED|disabled) +pos=#(-?\d+) +HM=\$([0-9a-fA-F]) +size=#(\d)`)
	reBall   = regexp.MustCompile(`^BL: (ENABLED|disabled) +pos=#(-?\d+) +HM=\$([0-9a-fA-F]) +size=#(\d)`)
	rePF     = regexp.MustCompile(`^PF0: %[01]{8}/\$([0-9a-fA-F]{2}) +PF1: %[01]{8}/\$([0-9a-fA-F]{2}) +PF2: %[01]{8}/\$([0-9a-fA-F]{2})`)
	reAud    = regexp.MustCompile(`^AUDF([01]): \$([0-9a-fA-F]{2})/.*AUDC[01]: \$([0-9a-fA-F]) +AUDV[01]: \$([0-9a-fA-F])`)
	reExec   = regexp.MustCompile(`^Executed \d+ commands from`)
)

// recordKeys are the line prefixes that START a logical record in the `tia`
// output. Anything else is a wrapped continuation of the previous record.
var recordKeys = []string{"scanline=", "Collisions:", "COLUxx:", "P0:", "P1:", "M0:", "M1:", "BL:", "PF0:", "inpt0", "AUDF0:", "AUDF1:"}

// ParseStellaSession extracts the `tia` command's output from a Stella debugger
// session file (saveSes) and returns the registers it reports.
//
// The prompt widget hard-wraps its output, and it wraps in two different ways:
// at exactly W columns it splits mid-token and consumes nothing, below W it broke
// at a space and consumed it. W is measured from the session itself — the
// autoexec banner `Executed N commands from "<long path>"` is always longer than
// the widget, so the length of its first physical line IS W.
func ParseStellaSession(session string) (TIARegs, error) {
	lines := strings.Split(strings.ReplaceAll(session, "\r\n", "\n"), "\n")

	width := 0
	for _, ln := range lines {
		if reExec.MatchString(ln) {
			width = len(ln)
			break
		}
	}
	if width < 40 {
		return nil, fmt.Errorf("cannot measure the prompt wrap width from this session (found %d); "+
			"is it a Stella saveSes file?", width)
	}

	// Slice out the block between the "> tia" prompt echo and the next prompt.
	start := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "> tia" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("no `> tia` command echo in the session — the tia command was never run")
	}
	var block []string
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "> ") {
			break
		}
		block = append(block, lines[i])
	}
	if len(block) == 0 {
		return nil, fmt.Errorf("the `tia` command produced no output")
	}

	// Unwrap into logical records.
	var records []string
	prevLen := 0
	for _, ln := range block {
		if strings.TrimSpace(ln) == "" {
			prevLen = len(ln)
			continue
		}
		isKey := false
		for _, k := range recordKeys {
			if strings.HasPrefix(ln, k) {
				isKey = true
				break
			}
		}
		if isKey || len(records) == 0 {
			records = append(records, ln)
		} else if prevLen == width {
			records[len(records)-1] += ln // hard wrap: nothing was consumed
		} else {
			records[len(records)-1] += " " + ln // word wrap: a space was consumed
		}
		prevLen = len(ln)
	}

	regs := TIARegs{}
	for _, rec := range records {
		if err := parseTIARecord(rec, regs); err != nil {
			return nil, err
		}
	}
	if _, ok := regs["COLUP0"]; !ok {
		return nil, fmt.Errorf("no COLUxx record in the tia output (%d records parsed)", len(records))
	}
	var missing []string
	for _, n := range TIARegNames {
		if _, ok := regs[n]; !ok {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("tia output did not yield %d register(s): %s", len(missing), strings.Join(missing, ","))
	}
	return regs, nil
}

// CaptureHeader is the provenance block scripts/stella_oracle.sh writes above a
// captured session: which ROM it came from and how many frames from power-on.
// Without it a stored session is an orphan number with nothing to compare to.
type CaptureHeader struct {
	ROM    string
	Frames int
}

var reHdr = regexp.MustCompile(`(?m)^# (rom|frames): *(.+?) *$`)

// ParseCaptureHeader reads the `# rom:` / `# frames:` provenance lines.
func ParseCaptureHeader(session string) (CaptureHeader, error) {
	var h CaptureHeader
	for _, m := range reHdr.FindAllStringSubmatch(session, -1) {
		switch m[1] {
		case "rom":
			h.ROM = m[2]
		case "frames":
			n, err := strconv.Atoi(m[2])
			if err != nil {
				return h, fmt.Errorf("bad `# frames:` header %q: %w", m[2], err)
			}
			h.Frames = n
		}
	}
	if h.ROM == "" || h.Frames <= 0 {
		return h, fmt.Errorf("capture has no `# rom:`/`# frames:` provenance header")
	}
	return h, nil
}

func hx(s string) int { v, _ := strconv.ParseUint(s, 16, 16); return int(v) }

// flag reports whether the UPPERCASE spelling of a flag word is present in the
// record. Stella writes a set flag in capitals and a clear one in lower case, so
// finding neither spelling is an error rather than a silent "clear".
func flag(rec, word string) (int, error) {
	up := regexp.MustCompile(`\b` + strings.ToUpper(word) + `\b`)
	lo := regexp.MustCompile(`\b` + strings.ToLower(word) + `\b`)
	switch {
	case up.MatchString(rec):
		return 1, nil
	case lo.MatchString(rec):
		return 0, nil
	}
	return 0, fmt.Errorf("neither %q nor %q found in %q", strings.ToUpper(word), strings.ToLower(word), rec)
}

func parseTIARecord(rec string, regs TIARegs) error {
	switch {
	case strings.HasPrefix(rec, "COLUxx:"):
		m := reColu.FindStringSubmatch(rec)
		if m == nil {
			return fmt.Errorf("unparsable COLUxx record: %q", rec)
		}
		regs["COLUP0"], regs["COLUP1"], regs["COLUPF"], regs["COLUBK"] = hx(m[1]), hx(m[2]), hx(m[3]), hx(m[4])

	case strings.HasPrefix(rec, "P0:"), strings.HasPrefix(rec, "P1:"):
		m := rePlayer.FindStringSubmatch(rec)
		if m == nil {
			return fmt.Errorf("unparsable player record: %q", rec)
		}
		n := m[1]
		gfx, err := strconv.ParseUint(m[2], 2, 16)
		if err != nil {
			return fmt.Errorf("player %s graphics: %w", n, err)
		}
		regs["GRP"+n] = int(gfx)
		regs["HMP"+n] = hx(m[4])
		mode := -1
		for i, lbl := range nusizPlayerLabels {
			if strings.Contains(rec, lbl) && (mode < 0 || len(lbl) > len(nusizPlayerLabels[mode])) {
				mode = i
			}
		}
		if mode < 0 {
			return fmt.Errorf("no known NUSIZ label in player record: %q", rec)
		}
		regs["NUSIZ"+n+"_PLAYER"] = mode
		v, err := flag(rec, "refl")
		if err != nil {
			return fmt.Errorf("player %s REFP: %w", n, err)
		}
		regs["REFP"+n] = v
		if v, err = flag(rec, "delay"); err != nil {
			return fmt.Errorf("player %s VDEL: %w", n, err)
		}
		regs["VDELP"+n] = v

	case strings.HasPrefix(rec, "M0:"), strings.HasPrefix(rec, "M1:"):
		m := reMissil.FindStringSubmatch(rec)
		if m == nil {
			return fmt.Errorf("unparsable missile record: %q", rec)
		}
		n := m[1]
		regs["ENAM"+n] = b2i(m[2] == "ENABLED")
		regs["HMM"+n] = hx(m[4])
		regs["NUSIZ"+n+"_MSIZE"], _ = strconv.Atoi(m[5])
		v, err := flag(rec, "reset")
		if err != nil {
			return fmt.Errorf("missile %s RESMP: %w", n, err)
		}
		regs["RESMP"+n] = v

	case strings.HasPrefix(rec, "BL:"):
		m := reBall.FindStringSubmatch(rec)
		if m == nil {
			return fmt.Errorf("unparsable ball record: %q", rec)
		}
		regs["ENABL"] = b2i(m[1] == "ENABLED")
		regs["HMBL"] = hx(m[3])
		regs["CTRLPF_BLSIZE"], _ = strconv.Atoi(m[4])
		v, err := flag(rec, "delay")
		if err != nil {
			return fmt.Errorf("ball VDELBL: %w", err)
		}
		regs["VDELBL"] = v

	case strings.HasPrefix(rec, "PF0:"):
		m := rePF.FindStringSubmatch(rec)
		if m == nil {
			return fmt.Errorf("unparsable playfield record: %q", rec)
		}
		regs["PF0"], regs["PF1"], regs["PF2"] = hx(m[1]), hx(m[2]), hx(m[3])
		for word, name := range map[string]string{"reflect": "CTRLPF_REFLECT", "score": "CTRLPF_SCORE", "priority": "CTRLPF_PRIORITY"} {
			v, err := flag(rec, word)
			if err != nil {
				return fmt.Errorf("CTRLPF: %w", err)
			}
			regs[name] = v
		}

	case strings.HasPrefix(rec, "AUDF"):
		m := reAud.FindStringSubmatch(rec)
		if m == nil {
			return fmt.Errorf("unparsable audio record: %q", rec)
		}
		n := m[1]
		regs["AUDF"+n], regs["AUDC"+n], regs["AUDV"+n] = hx(m[2]), hx(m[3]), hx(m[4])
	}
	return nil
}
