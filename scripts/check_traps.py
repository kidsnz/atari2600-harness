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
                                                   # ★that default is 31 files, NOT the whole corpus.
                                                   # The "123"/"171" figures in the comments below come
                                                   # from passing both directories explicitly:
                                                   #   check_traps.py roms/techniques/*.asm roms/litmus/*.asm
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
# The TIA's 14 read registers, $00-$0D. A read of any address in TIA space is decoded on
# the low nibble alone, so a write-only register read comes back as one of these unless the
# nibble is $0E or $0F. 〔Gopher2600 memorymap.go:96 maskReadTIA = 0x000f; :147〕
MIRROR_READ = {0x00: "CXM0P", 0x01: "CXM1P", 0x02: "CXP0FB", 0x03: "CXP1FB", 0x04: "CXM0FB",
               0x05: "CXM1FB", 0x06: "CXBLPF", 0x07: "CXPPMM", 0x08: "INPT0", 0x09: "INPT1",
               0x0A: "INPT2", 0x0B: "INPT3", 0x0C: "INPT4", 0x0D: "INPT5"}


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
    # ★2026-09-04: does this file target a mapper that decodes TIA space? Only then can a
    # NOP/BIT skip bankswitch. 3F/tigervision, 3E and X07 all do; a 4K or Fx cart does not.
    #
    # ★★The first version of this asked `\b(3f|3e|...)\b` of the whole file and got FIVE false
    # positives immediately: `$3F` is an ordinary byte in sprite data (`bitmap48.asm:211`,
    # `multicolor48.asm:241`), so a picture of a shape was read as a cartridge mapper. That is the
    # measuring-the-wrong-quantity error, in the gate that exists to catch mistakes. So: the name
    # must appear NOT as a hex literal, and on a line that is talking about cartridges.
    bankswitch_context = bool(
        re.search(r"\b(tigervision|x07)\b", asm, re.I)
        or re.search(r"(?im)^.*\b(cart(ridge)?|bank(switch(ing)?)?|mapper|scheme|hotspot)\b.*"
                     r"(?<![$\w])3[EF]\b.*$", asm)
        or re.search(r"(?im)^.*(?<![$\w])3[EF]\b.*"
                     r"\b(cart(ridge)?|bank(switch(ing)?)?|mapper|scheme|hotspot)\b.*$", asm))
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
        # Five spellings, three instructions, and only three of the five assemble.
        # Measured with DASM 2.20.14.1 on 2026-09-04 (found by the mailing-list distillation,
        # helper-1; re-run here): `asr`, `ane` and `lxa` are accepted; **`alr` and `xaa` are
        # rejected**, so they cannot appear in a .asm that builds. They stay in the pattern
        # anyway. Matching a mnemonic the assembler would refuse costs nothing — such a file is
        # already broken — while removing them makes this check specific to one assembler, and a
        # macro or a different tool could still emit them. The measurement is recorded here so
        # the next reader does not take it again: this file's own subject is a rule that was
        # re-derived from scratch because nobody wrote down that it had been settled.
        m = re.search(r"\b(lxa|xaa|ane|asr|alr)\b", low)
        if m:
            errors.append((n, f"unstable illegal opcode `{m.group(1)}` — HW-unreliable (use LAX/SAX/SBX/DCP instead)"))
        if re.search(r"\blax\s+#", low):
            errors.append((n, "`LAX #imm` (immediate) is the unstable LXA form — avoid"))
        # 2) NOP/BIT reading TIA space, used as a skip (spurious bankswitch on 3F/X07)
        #    〔known-traps C / mined 139089〕
        #
        # ★2026-09-04: this used to match `$00` ALONE, which is one address out of sixty-four.
        # The engine states the real condition in
        # `Gopher2600/hardware/memory/cartridge/mapper_tigervision.go` (alex_79, quoted there):
        #
        #     "The bankswitch happens if any address with both A6 and A7 low is accessed,
        #      and if A12 goes from low to high right after that access."
        #
        # and implements it as `addr&0x10c0 == 0x0000`. In zero page that is **$00-$3F, all
        # sixty-four of them** — `nop $04`, `bit $2C` and `nop $3F` were as dangerous as `nop $00`
        # and this said nothing about any of them. $40-$7F ($40 sets A6), $80-$BF ($80 sets A7)
        # and $C0-$FF are all outside it, which is why `NOP $80` is the recommended replacement.
        m = re.search(r"\b(nop|bit)\s+\$([0-9a-f]{1,2})\b", low)
        if m and (int(m.group(2), 16) & 0x10C0) == 0:
            warns.append((n, f"`{m.group(1).upper()} ${m.group(2).upper()}` reads TIA space (A6 and A7 both low, "
                             f"$00-$3F) and can trigger a bankswitch on 3F/X07 carts — use `NOP $80` "
                             f"or any address with A6 or A7 set"))
        # 2b) the same skip written as a RAW BYTE, which the mnemonic matcher above cannot see.
        #     ★The gap is real in this tree: `roms/techniques/tia_pcm.asm` skips with `.byte $2C`.
        #     $04/$0C/$14/$1C/$34/$3C/$44/$54/$64/$74/$80/$82/$89/$C2/$D4/$E2/$F4 are the NOP family
        #     and $24/$2C are BIT; the ones that TAKE AN OPERAND are the ones that read an address,
        #     so a raw-byte skip is exactly as capable of hitting TIA space as the spelled form.
        # ★★And it fires only where the hazard can exist. The bankswitch needs a mapper that
        #    decodes TIA space; a plain 4K cart has no such hardware, so warning there is noise,
        #    and a warning that fires on every correct use is a warning nobody reads. Measured
        #    2026-09-04: the unscoped version fired 8 times in this tree, all on 4K technique ROMs
        #    where the trap cannot happen. `bankswitch_context` is set once per file below.
        m = re.search(r"^\s*\.?byte\s+\$(04|0c|14|1c|24|2c|34|3c|44|54|64|74|d4|f4)\b", low)
        if m and bankswitch_context and "@skip-ok" not in raw:
            warns.append((n, f"`.byte ${m.group(1).upper()}` is a NOP/BIT skip written as a raw byte — the "
                             f"operand it swallows is READ, so if that address has A6 and A7 low it can "
                             f"bankswitch a 3F/X07 cart. Say why with `@skip-ok` if the address is safe"))
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
        #    Zero false positives measured 2026-07-30: 0 hits across the 123 files then in
        #    roms/techniques + roms/litmus (the read-side opcodes themselves match 509 times, so the
        #    detector is not silent — there really are none).
        #    RE-MEASURED 2026-09-03: 171 files (31 techniques + 140 litmus), still 0 ERROR, 1 warn.
        #      python3 scripts/check_traps.py roms/techniques/*.asm roms/litmus/*.asm
        #    Why this line changed: the sentence carried a count with NO date, so it read as a claim
        #    about the corpus as it stands. 48 litmus ROMs had been added since and nobody had
        #    re-run it — the claim happened to survive, but nothing was checking. Counts stated
        #    without a date are the failure mode the umbrella CLAUDE.md names; found by helper-3.
        m = READ_OP.search(code)
        if m:
            operand = m.group(2)
            reg = None
            v = -1
            if operand.upper() in TIA_WRITE_ONLY:
                reg = operand.upper()
                # ★2026-09-02: v was left unset on this branch when the mirror lookup was
                # added, so a symbolic operand (`lda GRP0`) would have read a stale v from a
                # previous line. Set it from the table, which is where the name came from.
                v = TIA_WRITE_ONLY[reg]
            elif operand.startswith("$"):
                try:
                    v = int(operand[1:], 16)
                except ValueError:
                    v = -1
                if 0x0E <= v <= 0x2C:
                    reg = next((k for k, a in TIA_WRITE_ONLY.items() if a == v), "$%02X" % v)
            if reg:
                # The reason line was wrong until 2026-09-02: it said the read "returns bus
                # residue". The TIA decodes reads on the low nibble only (the vendored engine
                # masks with 0x000f, memorymap.go:96/147), so of the 31 addresses $0E-$2C that
                # a program might read here, 27 land on a REAL read register and only four
                # ($0E $0F $1E $1F) are undriven. `lda GRP0` ($1B) returns INPT3, not residue;
                # `lda HMOVE` ($2A) returns INPT2; `lda CXCLR` ($2C) returns INPT4. That is
                # worse than residue, not better: the value is reproducible, so the bug hides.
                # Found by the Stella distillation (helper-3 computed it, reproduced here).
                mirror = MIRROR_READ.get(v & 0x0F)
                because = (f"reads back {mirror} (the TIA decodes reads on the low nibble only), "
                           f"so the value is REPRODUCIBLE and the bug hides"
                           if mirror else
                           "is one of the four undriven addresses ($0E $0F $1E $1F), so it "
                           "returns bus residue")
                errors.append((n, f"reads {reg}, a WRITE-ONLY TIA register — it {because}"))
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
        bit $2c           ; ★same trap, an address the old $00-only check could not see
        .byte $2c         ; ★same trap again, written as a raw byte (mnemonic matcher is blind)
flag    = $ff             ; var in stack-collision zone
        lda GRP0          ; read of a write-only TIA register
        sta $F123         ; write into cartridge ROM, undeclared
        ; (intentionally no CLD / CLEAN_START)
"""


def selftest():
    errors, warns = scan_text(BAIT)
    want = ["lxa", "LAX #imm", "bankswitch", "variable at $FF", "no CLD", "WRITE-ONLY TIA register",
            "cartridge ROM", "BIT $2C", "raw byte"]
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
