package oracle

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// captureDir holds one saved Stella debugger session per ROM (see
// scripts/stella_oracle.sh <rom> <frames> tia). Stella cannot be driven
// headlessly — its write-only TIA registers exist only in the debugger's TIA
// tab, which needs a GUI — so the reference answers are captured once, with a
// provenance header, and graded on every test run.
const captureDir = "testdata/stella_tia"

// minGradedROMs is the floor on how much of the corpus this check covers. It
// exists because the failure mode this project keeps hitting is not a wrong
// answer, it is a check that quietly stops covering anything: delete the
// captures and a "0 diffs" pass would otherwise look identical to a real one.
const minGradedROMs = 144

// minVariedRegisters is the same idea applied to the columns instead of the
// rows: a register that reads the same value in every ROM contributes to the
// count of "readings compared" without ever having been discriminated.
const minVariedRegisters = 20

// powerOnRAMROMs are the ROMs whose output depends on RAM that reset never
// wrote. Stella randomises power-on RAM (-plr.ramrandom, on by default) and
// Gopher2600 zeroes it, so Stella is not even reproducible here, let alone an
// oracle: two consecutive captures of uninit_trap reported COLUBK=$fc and
// COLUBK=$02 for the same ROM at the same frame. Stella is the one closer to
// hardware; that our machine reads a defined value is the very hazard these
// ROMs exist to demonstrate (see their headers, and defuse's UNINITIALISED
// READS). Named here rather than quietly dropped.
var powerOnRAMROMs = map[string]string{
	"uninit_trap.txt":        "reads $90, which reset never writes (the ROM's own header says so)",
	"litmus_uninit_read.txt": "reads RAM no path from reset wrote",
}

type capture struct {
	name    string
	header  CaptureHeader
	session string
}

func loadCaptures(t *testing.T) []capture {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(captureDir, "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no Stella captures in %s — the TIA write-register oracle is covering NOTHING. "+
			"Re-capture with: for r in roms/litmus/*.bin roms/techniques/*.bin; do "+
			"scripts/stella_oracle.sh $r 5 tia; done", captureDir)
	}
	sort.Strings(files)
	var out []capture
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		h, err := ParseCaptureHeader(string(b))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		out = append(out, capture{filepath.Base(f), h, string(b)})
	}
	return out
}

// TestStellaAgreesWithHarnessOnWriteOnlyTIARegisters is G4: the audit's gap was
// that COLUPF/NUSIZ and friends were never compared against a second
// implementation, only inferred from pixels. Every captured Stella session is
// re-graded here against a fresh Gopher2600 run of the same ROM at the same
// frame, register by register, and the counts are printed so a shrinking
// denominator is visible rather than silent.
func TestStellaAgreesWithHarnessOnWriteOnlyTIARegisters(t *testing.T) {
	caps := loadCaptures(t)

	totalReadings, totalDiffs, graded := 0, 0, 0
	// Witness: a register that reads the same value in every ROM was never
	// actually discriminated by this corpus, however big the denominator looks.
	// Count the distinct values each one takes so a vacuous column is visible.
	seen := map[string]map[int]bool{}
	var failures, explained []string
	for _, c := range caps {
		rom := filepath.Join("..", "..", c.header.ROM)
		if _, err := os.Stat(rom); err != nil {
			failures = append(failures, c.name+": ROM "+c.header.ROM+" is gone")
			continue
		}
		stella, err := ParseStellaSession(c.session)
		if err != nil {
			failures = append(failures, c.name+": "+err.Error())
			continue
		}
		ours, err := GopherTIARegs(rom, c.header.Frames)
		if err != nil {
			failures = append(failures, c.name+": "+err.Error())
			continue
		}
		diffs, compared, missing := DiffTIARegs(ours, stella)
		if len(missing) > 0 {
			failures = append(failures, c.name+": registers absent from a side: "+strings.Join(missing, ","))
			continue
		}
		graded++
		totalReadings += compared
		totalDiffs += len(diffs)
		for _, name := range TIARegNames {
			if seen[name] == nil {
				seen[name] = map[int]bool{}
			}
			seen[name][stella[name]] = true
		}
		if len(diffs) == 0 {
			continue
		}
		cls, err := ClassifyTIADiffs(rom, c.header.Frames, diffs)
		if err != nil {
			failures = append(failures, c.name+": "+err.Error())
			continue
		}
		ramReason, ramROM := powerOnRAMROMs[c.name]
		for _, cd := range cls {
			line := c.name + " (" + c.header.ROM + "): " + cd.String()
			switch {
			case cd.Class != TIADiffReal:
				explained = append(explained, line)
			case ramROM:
				explained = append(explained, c.name+" ("+c.header.ROM+"): "+cd.TIARegDiff.String()+
					" [power-on RAM: "+ramReason+"]")
			default:
				failures = append(failures, line)
			}
		}
	}

	var constant []string
	varied := 0
	for _, name := range TIARegNames {
		if len(seen[name]) <= 1 {
			constant = append(constant, name)
		} else {
			varied++
		}
	}
	t.Logf("graded %d/%d captured ROMs, %d register readings, %d disagreements (%d registers per ROM)",
		graded, len(caps), totalReadings, totalDiffs, len(TIARegNames))
	t.Logf("discrimination: %d/%d registers take more than one value across the corpus; constant: %v",
		varied, len(TIARegNames), constant)
	if len(explained) > 0 {
		t.Logf("%d disagreement(s) measured to be sampling phase / undefined power-on state, not divergence:\n  %s",
			len(explained), strings.Join(explained, "\n  "))
	}
	if varied < minVariedRegisters {
		t.Fatalf("only %d of %d registers vary across the corpus (want >= %d) — most of this "+
			"denominator is comparing a constant against a constant", varied, len(TIARegNames), minVariedRegisters)
	}
	if len(failures) > 0 {
		t.Errorf("Gopher2600 and Stella disagree on the write-only TIA registers:\n  %s",
			strings.Join(failures, "\n  "))
	}
	if graded < minGradedROMs {
		t.Errorf("only %d ROMs graded, want at least %d — the oracle's coverage has shrunk. "+
			"Anything between none and all is a defect, not a pass.", graded, minGradedROMs)
	}

	// ALL of the corpus, not "most of it": a ROM with no capture is a ROM this
	// oracle silently does not cover, and silence is the failure mode.
	var uncovered []string
	for _, dir := range []string{"../../roms/litmus", "../../roms/techniques"} {
		bins, err := filepath.Glob(filepath.Join(dir, "*.bin"))
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range bins {
			name := strings.TrimSuffix(filepath.Base(b), ".bin")
			if _, err := os.Stat(filepath.Join(captureDir, name+".txt")); err != nil {
				uncovered = append(uncovered, b)
			}
		}
	}
	// A ROM may be QUEUED rather than captured. Capturing needs Stella's GUI and takes
	// over the screen for ~13 s per ROM, which is not something to do in the middle of
	// someone's working session, so a ROM added mid-session is listed in CAPTURE_QUEUE
	// and captured in a batch later.
	//
	// The queue is not an exemption list, and it is built so that it cannot become one:
	// every queued line is PRINTED on every run, and the test fails once the queue
	// passes maxQueuedCaptures. A queue that stops being drained gets louder rather
	// than quieter — the opposite of the failure mode this whole file exists to avoid.
	queued, err := readCaptureQueue()
	if err != nil {
		t.Fatalf("reading %s: %v", queueFile, err)
	}
	var missing []string
	for _, b := range uncovered {
		if queued[strings.TrimPrefix(b, "../../")] {
			continue
		}
		missing = append(missing, b)
	}
	if len(missing) > 0 {
		t.Errorf("%d corpus ROM(s) have no Stella capture and are not queued, so this oracle "+
			"does not cover them: %v\ncapture each with: scripts/stella_oracle.sh <rom> 5 tia\n"+
			"or add a line to %s if the machine is in use", len(missing), missing, queueFile)
	}
	if len(queued) > 0 {
		names := make([]string, 0, len(queued))
		for q := range queued {
			names = append(names, q)
		}
		sort.Strings(names)
		t.Logf("%d ROM(s) awaiting capture (drain with scripts/stella_oracle.sh <rom> 5 tia):", len(queued))
		for _, n := range names {
			t.Logf("    %s", n)
		}
	}
	if len(queued) > maxQueuedCaptures {
		t.Errorf("%d ROMs are queued for capture, over the limit of %d — the queue exists so a "+
			"ROM added mid-session does not block work, not so captures can be skipped "+
			"indefinitely. Drain it.", len(queued), maxQueuedCaptures)
	}
}

// queueFile lists ROMs whose capture is deferred; see the file's own header.
const queueFile = captureDir + "/CAPTURE_QUEUE"

// maxQueuedCaptures is how many ROMs may await capture before the queue itself is the
// defect. Small on purpose: the queue is for the hour between adding a ROM and the
// user stepping away from the machine, not for a backlog.
const maxQueuedCaptures = 6

func readCaptureQueue() (map[string]bool, error) {
	b, err := os.ReadFile(queueFile)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	out := map[string]bool{}
	for _, ln := range strings.Split(string(b), "\n") {
		if i := strings.Index(ln, "#"); i >= 0 {
			ln = ln[:i]
		}
		if ln = strings.TrimSpace(ln); ln != "" {
			out[ln] = true
		}
	}
	return out, nil
}

// TestPlantedDefectInARecoveredRegisterIsCaught proves the comparison can FAIL.
// A check that has never been seen to fail is not evidence of anything, so one
// recovered register is corrupted by a single bit and the diff must name it.
func TestPlantedDefectInARecoveredRegisterIsCaught(t *testing.T) {
	caps := loadCaptures(t)
	// Pick the richest capture: flipping a bit in a register that is already 0
	// everywhere would demonstrate very little.
	c := caps[0]
	best := -1
	for _, cand := range caps {
		regs, err := ParseStellaSession(cand.session)
		if err != nil {
			continue
		}
		n := 0
		for _, name := range TIARegNames {
			if regs[name] != 0 {
				n++
			}
		}
		if n > best {
			best, c = n, cand
		}
	}
	stella, err := ParseStellaSession(c.session)
	if err != nil {
		t.Fatal(err)
	}
	ours, err := GopherTIARegs(filepath.Join("..", "..", c.header.ROM), c.header.Frames)
	if err != nil {
		t.Fatal(err)
	}
	if d, _, _ := DiffTIARegs(ours, stella); len(d) != 0 {
		t.Fatalf("baseline must agree before a defect is planted, got %v", d)
	}
	for _, reg := range []string{"COLUPF", "NUSIZ0_PLAYER", "HMP0", "PF1", "VDELP0"} {
		planted := TIARegs{}
		for k, v := range ours {
			planted[k] = v
		}
		planted[reg] = ours[reg] ^ 1 // flip one bit
		diffs, compared, _ := DiffTIARegs(planted, stella)
		if len(diffs) != 1 || diffs[0].Reg != reg {
			t.Fatalf("planted defect in %s was not caught (compared %d, diffs %v)", reg, compared, diffs)
		}
		t.Logf("planted defect caught: %s", diffs[0])
	}
}

// TestTheClassifierCannotExcuseAPlantedDefect: ClassifyTIADiffs exists to keep
// sampling phase and undefined power-on state out of the failure list, which is
// exactly the shape of an excuse that grows until nothing can fail. So plant a
// defect on the probe ROM — every one of whose registers is written once and
// then left alone, so no instant of any frame holds a different value — and
// require the classifier to call it a divergence.
func TestTheClassifierCannotExcuseAPlantedDefect(t *testing.T) {
	const rom = "testdata/tiaprobe.bin"
	b, err := os.ReadFile(filepath.Join(captureDir, "tiaprobe.txt"))
	if err != nil {
		t.Fatal(err)
	}
	stella, err := ParseStellaSession(string(b))
	if err != nil {
		t.Fatal(err)
	}
	for _, reg := range []string{"COLUPF", "NUSIZ0_PLAYER", "HMP0", "PF1", "VDELP0", "GRP1"} {
		// Stella is made to report a value our side holds at no instant of the
		// frame. That is what an unexplainable disagreement looks like.
		planted := TIARegDiff{Reg: reg, Ours: stella[reg], Stella: stella[reg] ^ 1}
		cls, err := ClassifyTIADiffs(rom, 5, []TIARegDiff{planted})
		if err != nil {
			t.Fatal(err)
		}
		if len(cls) != 1 || cls[0].Class != TIADiffReal {
			t.Fatalf("planted %s defect was excused as %q — the classifier is swallowing failures",
				reg, cls[0].Class)
		}
		t.Logf("classifier calls it a divergence: %s", cls[0])
	}
}

// TestStellaCaptureConventionsMatchTheProbeROMsSource locks the meaning of every
// field of Stella's `tia` text against the constants testdata/tiaprobe*.asm
// write. The expectations below come from those .asm files, NOT from Gopher2600
// — if they came from our own emulator, agreeing with it later would prove
// nothing.
func TestStellaCaptureConventionsMatchTheProbeROMsSource(t *testing.T) {
	want := map[string]map[string]int{
		// tiaprobe.asm
		"tiaprobe.txt": {
			"COLUP0": 0x32, "COLUP1": 0x54, "COLUPF": 0x76, "COLUBK": 0x98,
			"GRP0": 0xA5, "GRP1": 0x3C, // GRP0 old is $22: Stella must print the NEW copy
			"NUSIZ0_PLAYER": 6, "NUSIZ1_PLAYER": 5, "NUSIZ0_MSIZE": 1, "NUSIZ1_MSIZE": 2,
			"CTRLPF_REFLECT": 1, "CTRLPF_SCORE": 1, "CTRLPF_PRIORITY": 1, "CTRLPF_BLSIZE": 2,
			"REFP0": 1, "REFP1": 0, "VDELP0": 1, "VDELP1": 0, "VDELBL": 1,
			"ENAM0": 1, "ENAM1": 0, "ENABL": 1, "RESMP0": 1,
			"PF0": 0xB, "PF1": 0xC3, "PF2": 0x5A,
			"HMP0": 7, "HMP1": 0xF, "HMM0": 8, "HMM1": 1, "HMBL": 0xD,
		},
		// tiaprobe2.asm — every asymmetric choice mirrored
		"tiaprobe2.txt": {
			"COLUP0": 0x1A, "COLUP1": 0x2C, "COLUPF": 0x4E, "COLUBK": 0x60,
			"GRP0": 0x81, "GRP1": 0x5A,
			"NUSIZ0_PLAYER": 3, "NUSIZ1_PLAYER": 7, "NUSIZ0_MSIZE": 3, "NUSIZ1_MSIZE": 0,
			"CTRLPF_REFLECT": 0, "CTRLPF_SCORE": 1, "CTRLPF_PRIORITY": 0, "CTRLPF_BLSIZE": 1,
			"REFP0": 0, "REFP1": 1, "VDELP0": 0, "VDELP1": 1, "VDELBL": 0,
			"ENAM0": 0, "ENAM1": 1, "ENABL": 0, "RESMP0": 0,
			"PF0": 0x7, "PF1": 0x0F, "PF2": 0xA5,
			"HMP0": 3, "HMP1": 9, "HMM0": 0xB, "HMM1": 0xE, "HMBL": 0,
		},
	}
	for name, exp := range want {
		b, err := os.ReadFile(filepath.Join(captureDir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got, err := ParseStellaSession(string(b))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for reg, v := range exp {
			if got[reg] != v {
				t.Errorf("%s: Stella reports %s=%d ($%02x), the ROM source writes %d ($%02x)",
					name, reg, got[reg], got[reg], v, v)
			}
		}
	}
}

// TestStella70MisreportsRESMP1 is the one disagreement the corpus turned up, and
// it is Stella's. Its M1 line's reset flag tracks RESMP0, never RESMP1:
// tiaprobe writes RESMP0=$02/RESMP1=$00 and Stella prints the flag SET on both
// missile lines; tiaprobe2 writes the mirror and Stella prints it CLEAR on both.
// So RESMP1 is excluded from TIARegNames — and locked here, so that if a later
// Stella fixes it this test fails and the register can be put back.
func TestStella70MisreportsRESMP1(t *testing.T) {
	for name, resmp0 := range map[string]int{"tiaprobe.txt": 1, "tiaprobe2.txt": 0} {
		b, err := os.ReadFile(filepath.Join(captureDir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		regs, err := ParseStellaSession(string(b))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if regs["RESMP0"] != resmp0 {
			t.Fatalf("%s: RESMP0 should read %d (the ROM writes it)", name, resmp0)
		}
		if regs["RESMP1"] != resmp0 {
			t.Fatalf("%s: Stella now reports RESMP1=%d instead of mirroring RESMP0=%d — "+
				"the 7.0 defect is fixed, put RESMP1 back into TIARegNames",
				name, regs["RESMP1"], resmp0)
		}
	}
	if _, excluded := TIARegsNotReported["RESMP1"]; !excluded {
		t.Fatal("RESMP1 must stay named in TIARegsNotReported while Stella misreports it")
	}
	for _, n := range TIARegNames {
		if n == "RESMP1" {
			t.Fatal("RESMP1 is compared against a Stella field that does not report it")
		}
	}
}

// TestStellaSessionParserRejectsRubbish: the parser must refuse what it cannot
// read instead of returning a half-filled reading that would silently shrink the
// comparison.
func TestStellaSessionParserRejectsRubbish(t *testing.T) {
	cases := map[string]string{
		"empty":            "",
		"no wrap beacon":   "> tia\nCOLUxx: P0=$00/ P1=$00/ PF=$00/ BK=$00/\n",
		"no tia command":   "Executed 2 commands from \"~/Library/Application Support/Stella/autoexec.script\"\n> ram\n",
		"truncated block":  "Executed 2 commands from \"~/Library/Application Support/Stella/autoexec.script\"\n> tia\nCOLUxx: P0=$12/      P1=$34/      PF=$56/      BK=$78/\n> saveSes\n",
		"corrupt colu row": "Executed 2 commands from \"~/Library/Application Support/Stella/autoexec.script\"\n> tia\nCOLUxx: P0=$zz/\n> saveSes\n",
	}
	for name, s := range cases {
		if _, err := ParseStellaSession(s); err == nil {
			t.Errorf("%s: parser accepted a session it should have rejected", name)
		}
	}
}
