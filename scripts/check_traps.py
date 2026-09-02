#!/usr/bin/env python3
"""check_traps.py — a static linter for the "dies on real hardware" traps, run before a kernel ships.

Detects the **statically decidable** traps from `docs/known-traps.md` in generated or hand-written
.asm (preflight step 2 of [[feedback-authoring-loop-system]]; the pre-Pong gate of
[[project-roadmap-to-pong-capstone]]).
Runtime-only traps (RIOT timer wraparound and the like) are the responsibility of the
scenario/`breakif` side = not handled here.

Usage:
    cd harness
    python3 scripts/check_traps.py [file.asm ...]   # with no argument, checks roms/techniques/*.asm
    python3 scripts/check_traps.py --selftest        # self-test of the detectors (bait string confirms every detector fires)

Verdict: a single ERROR means exit 1 (fails CI). WARN is informational (does not affect the exit).
Zero false positives is the top priority (the existing roms/techniques are all clean). Low-confidence
traps stop at WARN.
"""
import glob
import os
import re
import sys

HARNESS = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))


def strip_comment(line):
    # Drop everything from `;` onward (string literals are rare in 2600 asm, so this stays simple).
    i = line.find(";")
    return (line[:i] if i >= 0 else line)



# The TIA's "write-only" registers = $0E(PF1)-$2C(CXCLR). On the read side there is only $00-$0D
# (Gopher2600 cpubus.go TIAReadRegisters is just CXM0P..INPT5, $00-$0D). Reading there returns bus
# residue rather than the register = the classic "happens to work in the emulator".
TIA_WRITE_ONLY = {
    "PF1": 0x0E, "PF2": 0x0F, "RESP0": 0x10, "RESP1": 0x11, "RESM0": 0x12, "RESM1": 0x13,
    "RESBL": 0x14, "AUDC0": 0x15, "AUDC1": 0x16, "AUDF0": 0x17, "AUDF1": 0x18, "AUDV0": 0x19,
    "AUDV1": 0x1A, "GRP0": 0x1B, "GRP1": 0x1C, "ENAM0": 0x1D, "ENAM1": 0x1E, "ENABL": 0x1F,
    "HMP0": 0x20, "HMP1": 0x21, "HMM0": 0x22, "HMM1": 0x23, "HMBL": 0x24, "VDELP0": 0x25,
    "VDELP1": 0x26, "VDELBL": 0x27, "RESMP0": 0x28, "RESMP1": 0x29, "HMOVE": 0x2A,
    "HMCLR": 0x2B, "CXCLR": 0x2C,
}
# Writes into cartridge space ($1000-$1FFF and the $F000-$FFFF mirror). ROM cannot be written, so
# the value disappears — the only exceptions are bank-switch hotspots and the SuperChip write port,
# and both of those are something you do ON PURPOSE. Intent cannot be guessed, so it has to be
# declared: `@rom-write-ok` at the end of the line.
# Measured 2026-07-30: across the 123 files in roms/techniques + roms/litmus there were only 2
# matches, and both are lines where litmus_6502 aims at ROM deliberately in order to measure that
# "STA abs,X is a fixed 5cy even across a page boundary" — the source comment says so too. So the
# default was made ERROR and those 2 lines got the declaration.
STORE_OP = re.compile(r"\b(sta|stx|sty)\s+\$([0-9a-fA-F]{3,4})", re.I)

READ_OP = re.compile(
    r"\b(lda|ldx|ldy|bit|cmp|cpx|cpy|adc|sbc|and|ora|eor)\s+(?!#)(\$?[0-9a-zA-Z_]+)", re.I)

def scan_text(asm):
    """Check an asm string and return (errors, warns), each a list of (line number, message)."""
    errors, warns = [], []
    has_cld = has_cleanstart = False
    lines = asm.splitlines()
    for n, raw in enumerate(lines, 1):
        code = strip_comment(raw)
        low = code.lower()
        if re.search(r"\bcld\b", low):
            has_cld = True
        if re.search(r"clean_start", low):
            has_cleanstart = True
        # 1) Unstable illegal opcodes (break on real hardware depending on the unit/temperature) 〔known-traps D / mined 168616,132496〕
        # ASR/ALR joined this list on 2026-09-02. The docs had it in the STABLE set citing
        # mining 168616, but 168616 itself reports it failing on official hardware (late
        # Taiwanese Atari Jr; Thunderground's score corrupts) and 294471 s32 carries an
        # independent confirmation from omegamatrix on real hardware. No ROM in the corpus
        # uses it (measured with two structurally different expressions, both exit 1), so
        # adding it costs nothing and closes the gap between the docs and this check.
        m = re.search(r"\b(lxa|xaa|ane|asr|alr)\b", low)
        if m:
            errors.append((n, f"unstable illegal opcode `{m.group(1)}` — HW-unreliable (use LAX/SAX/SBX/DCP instead)"))
        if re.search(r"\blax\s+#", low):
            errors.append((n, "`LAX #imm` (immediate) is the unstable LXA form — avoid"))
        # 2) NOP $00 / BIT $00 used as a skip (spurious bankswitch on 3F/X07) 〔known-traps C / mined 139089〕
        if re.search(r"\b(nop|bit)\s+\$00\b", low):
            warns.append((n, "`NOP $00`/`BIT $00` can trigger a bankswitch on 3F/X07 carts — use `NOP $80` or a safe address"))
        # 3) Variable assigned into the stack-collision zone ($F8-$FF) 〔known-traps C / mined 302998,301766〕
        m = re.search(r"=\s*\$(f[89a-f])\b", low) or re.search(r"\bequ\s+\$(f[89a-f])\b", low)
        if m:
            warns.append((n, f"variable at $%s — JSR pushes onto the $0100/$00FF stack mirror and can clobber it (keep vars from $80)" % m.group(1).upper()))
        # 6) Writes into ROM (with no declaration) 〔known-traps C / mined 285759,204819〕
        m = STORE_OP.search(code)
        if m and "@rom-write-ok" not in raw:
            a = int(m.group(2), 16)
            if 0x1000 <= a <= 0x1FFF or 0xF000 <= a <= 0xFFFF:
                errors.append((n, f"stores to ${a:04X}, which is cartridge ROM — the write is discarded. "
                                  f"If it is a bank-switch hotspot or a SuperChip write port, say so with "
                                  f"`; @rom-write-ok` so the intent is declared rather than guessed"))
        # 5) Reads of a write-only TIA register 〔known-traps / Gopher2600 cpubus.TIAReadRegisters=$00-$0D〕
        #    Zero false positives measured: 0 hits across the 123 files in roms/techniques + roms/litmus
        #    (the read-side opcodes themselves match 509 times, so the detector is not silent — there
        #    really are none).
        m = READ_OP.search(code)
        if m:
            operand = m.group(2)
            reg = None
            if operand.upper() in TIA_WRITE_ONLY:
                reg = operand.upper()
            elif operand.startswith("$"):
                try:
                    v = int(operand[1:], 16)
                except ValueError:
                    v = -1
                if 0x0E <= v <= 0x2C:
                    reg = next((k for k, a in TIA_WRITE_ONLY.items() if a == v), "$%02X" % v)
            if reg:
                errors.append((n, f"reads {reg}, a WRITE-ONLY TIA register — the TIA answers reads only at "
                                  f"$00-$0D (CXxx/INPTx); this returns bus residue, which an emulator may "
                                  f"make look deterministic"))
    # 4) Reset initialisation (neither CLD nor CLEAN_START) 〔known-traps D / mined 261488,318346〕
    if not (has_cld or has_cleanstart):
        errors.append((0, "no CLD and no CLEAN_START — decimal flag / SP / RAM are undefined at power-up (BCD garbage, rolls)"))
    return errors, warns


def check_file(path):
    with open(path, encoding="utf-8", errors="ignore") as f:
        return scan_text(f.read())


# --- Self-test bait (every detector must fire at least once) ---
BAIT = """
        processor 6502
Start
        lxa #$00          ; unstable illegal opcode
        lax #$ff          ; immediate LAX = unstable
        nop $00           ; bankswitch trap on 3F
flag    = $ff             ; var in stack-collision zone
        lda GRP0          ; read of a write-only TIA register
        sta $F123         ; write into cartridge ROM, undeclared
        ; (intentionally no CLD / CLEAN_START)
"""


def selftest():
    errors, warns = scan_text(BAIT)
    want = ["lxa", "LAX #imm", "bankswitch", "variable at $FF", "no CLD", "WRITE-ONLY TIA register", "cartridge ROM"]
    blob = " ".join(m for _, m in errors + warns)
    missing = [w for w in want if w not in blob]
    if missing:
        print("SELFTEST FAIL — detectors didn't fire for:", missing)
        for ln, m in errors + warns:
            print("  got:", m)
        sys.exit(1)
    print("selftest OK — all %d trap detectors fire on the bait" % len(want))


def main():
    if "--selftest" in sys.argv:
        selftest()
        return
    files = [a for a in sys.argv[1:] if not a.startswith("-")]
    if not files:
        files = sorted(glob.glob(os.path.join(HARNESS, "roms", "techniques", "*.asm")))
    n_err = 0
    for p in files:
        errs, warns = check_file(p)
        rel = os.path.relpath(p, HARNESS)
        for ln, m in errs:
            print(f"ERROR {rel}:{ln}: {m}")
            n_err += 1
        for ln, m in warns:
            print(f"warn  {rel}:{ln}: {m}")
    if n_err:
        print(f"\n{n_err} trap error(s) — see docs/known-traps.md. (rule: feedback-authoring-loop-system)")
        sys.exit(1)
    print("traps OK — no emu-passes/HW-fails static traps in %d asm file(s)." % len(files))


if __name__ == "__main__":
    main()
