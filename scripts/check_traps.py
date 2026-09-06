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

# Every 6502 mnemonic, legal and illegal, at the start of a statement. Used only to answer
# "does this file contain any code at all?" — a data-only include is not an init routine.
INSTRUCTION = re.compile(
    r"^\s*(?:\w+:?\s+)?("
    r"adc|and|asl|bcc|bcs|beq|bit|bmi|bne|bpl|brk|bvc|bvs|clc|cld|cli|clv|cmp|cpx|cpy|dec|dex|"
    r"dey|eor|inc|inx|iny|jmp|jsr|lda|ldx|ldy|lsr|nop|ora|pha|php|pla|plp|rol|ror|rti|rts|sbc|"
    r"sec|sed|sei|sta|stx|sty|tax|tay|tsx|txa|txs|tya|"
    r"lax|sax|dcp|isb|slo|rla|sre|rra|anc|alr|arr|sbx|las|lxa|axs|asr|jam|kil"
    r")\b",
    re.I | re.M)

READ_OP = re.compile(
    r"\b(lda|ldx|ldy|bit|cmp|cpx|cpy|adc|sbc|and|ora|eor)\s+(?!#)(\$?[0-9a-zA-Z_]+)", re.I)

def addressed_as_memory(asm, name):
    """Is `name` ever used as an ADDRESS, rather than only as a value?

    ★2026-09-05. Rule 3 warns about a variable placed in $F8-$FF, where JSR's stack push can
    clobber it. It matched the assignment alone, so every `NAME = $FC` looked like a variable —
    and an EQU's right-hand side is as often a value: `COLM1 = $FC ; the gold electron` is a
    colour, `M1 = $FF` is a NUSIZ constant. A colour is written `lda #COLM1`; a variable is
    written `sta COLM1`. The `#` is the whole difference, and it is in the operand, not in the
    assignment — so the answer needs the rest of the file. Found when the gate was first aimed at
    the author's own work (helper-2) and this was the single warning that survived the first fix.
    """
    return re.search(
        r"\b(?:lda|ldx|ldy|sta|stx|sty|inc|dec|asl|lsr|rol|ror|bit|cmp|cpx|cpy|adc|sbc|and|ora|eor)"
        r"\s+(?!#)\(?" + re.escape(name) + r"\b", asm, re.I) is not None


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
    # ★One fact about the file as a whole, needed by rule 4 below.
    #
    # ★★A second one was here for an hour and was removed the same day: `pushes`, "does this
    # file contain a JSR/PHA/PHP", used to gate rule 3 on the theory that a stack collision
    # needs a push. Two things killed it. Mutation-testing the four new guards against the
    # selftest showed three were caught and this one was NOT — no counter-bait distinguished
    # it, because the clean sample happens to contain a `jsr`. And it is unsound anyway: these
    # works are built from includes, so the file that declares a variable and the file that
    # pushes are routinely different files. The usage test (`addressed_as_memory`) alone takes
    # the whole tree — both works, sandbox and the 31 technique ROMs — to zero warnings, so the
    # unsound guard was not even carrying its weight.
    has_instructions = bool(INSTRUCTION.search(asm))
    defines_reset_vector = bool(re.search(r"\$fffc\b", asm, re.I))
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
            errors.append((n, f"unstable illegal opcode `{m.group(1)}` — HW-unreliable. `LAX/SAX/SBX/DCP` are the stable "
                            f"family ON ORIGINAL NMOS SILICON; `SBX` and `ARR` are reported to fail on the "
                            f"Flashback 2 (a reimplementation, not a 6507), so scope the claim if the target "
                            f"is not an original console — see docs/known-traps.md"))
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
        # 2c) DASM's SLEEP macro, which hides the same bytes behind a macro call.
        #     ★Measured 2026-09-05 by assembling dasm 2.20.14.1's machines/atari2600/macro.h:
        #        SLEEP 2 -> EA          nop                      2 cy, legal, touches nothing
        #        SLEEP 3 -> 04 00       nop $00   ILLEGAL opcode, and READS $00
        #        SLEEP 4 -> EA EA
        #        SLEEP 5 -> 04 00 EA
        #     and with -DNO_ILLEGAL_OPCODES=1 the 04 becomes 24 (`bit $00`) — legal, SAME ADDRESS.
        #     ★★So the switch everyone recommends fixes the opcode and leaves the bankswitch trap.
        #     Even values are safe: they are plain `nop`s. This is the answer to Kirk Israel's
        #     question in 200403 ("if it's a multiple of 2, isn't SLEEP safe?"), which nobody in
        #     that thread answered. Hence a CONDITION, not a ban.
        # ★`n` is the LINE NUMBER in the enclosing loop; the SLEEP value gets its own name.
        #   Shadowing it made the first version report the wrong line, which is the quiet kind of
        #   wrong a linter must not be.
        m = re.search(r"\bsleep\s+#?\$?([0-9a-fx]+)\b", low)
        if m:
            tok = m.group(1)
            try:
                val = int(tok[1:], 16) if tok.startswith("$") else int(tok, 0)
            except ValueError:
                val = None
            if val is not None and val >= 3 and val % 2 == 1:
                warns.append((n, f"`SLEEP {val}` is ODD, so it emits a read of `$00` — `nop $00` "
                                 f"(illegal) by default, `bit $00` (legal, but CHANGES FLAGS and "
                                 f"reads the SAME address) under NO_ILLEGAL_OPCODES. Either can "
                                 f"bankswitch a 3F/X07 cart. Even values are plain NOPs and are "
                                 f"fine. ★There is no legal 3-cycle filler that touches no memory, "
                                 f"which is why this is a warning and not an error: the macro's own "
                                 f"author listed all three forms in 200207 and all three read $00. "
                                 f"If flags are expendable, `bit $00` is the cheapest; if they are "
                                 f"not, `PHP`/`PLP` costs 7 cycles in 2 bytes and touches nothing "
                                 f"(measured — internal/emu/oddsleep_test.go)"))

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
        #    ★2026-09-05: only when the symbol is used as an ADDRESS. `NAME = $FC` is an EQU,
        #    and an EQU's right-hand side is as often a value as an address — `COLM1 = $FC ;
        #    the gold electron` is a colour, `M1 = $FF` is a NUSIZ constant. A colour is read
        #    `lda #COLM1`; a variable is read `sta COLM1`. See `addressed_as_memory`. Measured:
        #    this takes the ~16 warnings across both works, sandbox and the 31 technique ROMs
        #    to **zero**, and every one of the 16 was an EQU used only behind a `#`.
        m = re.search(r"=\s*\$(f[89a-f])\b", low) or re.search(r"\bequ\s+\$(f[89a-f])\b", low)
        if m:
            name = re.match(r"\s*(\w+)", code)  # the symbol being assigned
            if name is None or addressed_as_memory(asm, name.group(1)):
                warns.append((n, f"variable at $%s — JSR pushes onto the $0100/$00FF stack mirror and can clobber it (keep vars from $80)" % m.group(1).upper()))
        # 6) Writes into ROM (with no declaration) 〔known-traps C / mined 285759,204819〕
        m = STORE_OP.search(code)
        if m and "@rom-write-ok" not in raw:
            a = int(m.group(2), 16)
            if 0x1000 <= a <= 0x1FFF or 0xF000 <= a <= 0xFFFF:
                errors.append((n, f"stores to ${a:04X}, which is cartridge ROM — the write is discarded. "
                                  f"If it is a bank-switch hotspot or a SuperChip write port, say so with "
                                  f"`; @rom-write-ok` so the intent is declared rather than guessed"))
        # 6b) The SAME cartridge page, reached by an address the author never wrote.
        #     ★An indexed store performs a PHANTOM READ before the real write, and the engine's own
        #     comment says when: `hardware/cpu/cpu.go`, "phantom read (always happens for Write and
        #     Modify)", at **`(base & 0xFF00) | (effective & 0x00FF)`** — the base's page with the
        #     effective address's low byte. So `sta $1F00,X` reads `$1F00 | (eff & $FF)`, and if the
        #     index puts `$F8` there that read lands on **`$1FF8`, an F8 bank-switch hotspot**.
        #     ★★The upper half of that address comes from the BASE, so this is decidable without
        #     resolving the index: a base in cartridge space means the phantom read is in cartridge
        #     space too, and some index value reaches every offset in the page. The rule is therefore
        #     conservative — it says "can", not "does" — and rule 6 above already provides the way to
        #     declare intent.
        #     ★★★Rule 6 catches the write the author WROTE. This catches the read the author did
        #     not. Found by the AtariAge cross-check (helper-2) against stella-list's own report of
        #     the same mechanism from the other side (Eckhard Stolberg on indexed writes crossing a
        #     page), which is the first place both corpora carried one fact.
        m = re.search(r"\b(sta|stx|sty|inc|dec|asl|lsr|rol|ror)\s+\$([0-9a-f]{4})\s*,\s*[xy]\b", low)
        if m and "@rom-write-ok" not in raw:
            base = int(m.group(2), 16)
            if 0x1000 <= base <= 0x1FFF or 0xF000 <= base <= 0xFFFF:
                warns.append((n, f"`{m.group(1).upper()} ${base:04X},{'X' if ',x' in low else 'Y'}` — the "
                                 f"indexed store's PHANTOM READ lands at ${base & 0xFF00:04X} plus the "
                                 f"effective low byte, so some index value reaches every address in that "
                                 f"cartridge page, hotspots included. The read happens on EVERY indexed "
                                 f"store, not only on a page cross. Declare it with `; @rom-write-ok` if "
                                 f"the page is known to hold none"))

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
            # ★2026-09-05: the name match is CASE-SENSITIVE, and that is the whole rule.
            #   This compared `operand.upper()`, so `lda pf1` — a RAM variable, `pf1 = MUSZP+7`
            #   in the music driver — was read as `lda PF1`, the TIA register. Pointing this
            #   gate at the author's first work for the first time produced **112** errors,
            #   56 PF1 + 56 PF2, every one of them that. Measured before changing it: in this
            #   whole tree the register names are written in upper case (WSYNC 15438, GRP0
            #   10261, GRP1 7191, VBLANK 1783, PF1 1399) and the only lower-case occurrences of
            #   a register-shaped name are `pf0`/`pf1`/`pf2`, 254 each, all of them that one
            #   work's RAM variables. No `.asm` here writes a register name in lower case.
            #   Found by the mailing-list distillation (helper-2), who aimed the gate at the
            #   works and then opened all 112.
            if operand in TIA_WRITE_ONLY:
                reg = operand
                # ★2026-09-02: v was left unset on this branch when the mirror lookup was
                # added, so a symbolic operand (`lda GRP0`) would have read a stale v from a
                # previous line. Set it from the table, which is where the name came from.
                v = TIA_WRITE_ONLY[reg]
            elif operand.startswith("$"):
                try:
                    v = int(operand[1:], 16)
                except ValueError:
                    v = -1
                # ★2026-09-05: fold the address the way the machine does before deciding.
                #   The TIA occupies $00-$3F and `memorymap.go:95` masks WRITES with
                #   `maskWriteTIA = MemtopTIA` = **$3F** — so `$02`, `$42`, `$82` and `$C2` are all
                #   WSYNC, and `$5B` is GRP0 exactly as `$1B` is. This rule compared the raw
                #   operand, so **`lda $1B` fired and `lda $5B` did not** (measured). A kernel
                #   written against mirrors slipped past every address-based rule here.
                #   ★★The read mask is a different number ($0F, used below for what comes BACK);
                #   this one decides WHICH register the address names. Found by the AtariAge
                #   cross-check (helper-2), who also warned that the two masks are not
                #   interchangeable.
                #   ★★★And the fold must not be greedy. RAM is $80-$FF and `MapAddress` tests
                #   `address & OriginRAM == OriginRAM` (bit 7) BEFORE falling through to TIA, so
                #   **$80-$FF is RAM, not a TIA mirror**. The first version of this folded every
                #   address and immediately reported `lda $8E` in `game_states.asm` as "reads PF1"
                #   — twice. It is a RAM read. Fold only when bit 7 is clear.
                if 0 <= v <= 0x7F:
                    v &= 0x3F
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
    #    ★2026-09-05: only for a file that IS a whole program. "The decimal flag is undefined at
    #    power-up" is a claim about the RESET entry, so the rule belongs to the file that defines
    #    the reset vector — and only if that file also contains code.
    #
    #    Both halves were measured, and each one alone was wrong. A tree of 411 `.asm`:
    #      * 121 + 2 + … files are INCLUDES with code and no vector — `transistor-sp4x-body.asm`
    #        is a generated, fully-unrolled kernel body whose `CLD` lives in the file that
    #        includes it. Requiring instructions alone still fired on 35 of these.
    #      * four files are the opposite — `tables-*.asm` in the second work carry
    #        `org $FFFC / .word Start` and **zero** instructions, the tail-include idiom.
    #        Requiring the vector alone fired on all four.
    #    Requiring both: 266 files define the vector, 262 of them have CLD or CLEAN_START, and
    #    the four that do not are exactly the vector-only includes. The tree goes to zero, and
    #    nothing that is genuinely a program is excused.
    if not (has_instructions and defines_reset_vector):
        pass
    elif not (has_cld or has_cleanstart):
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
        SLEEP 3           ; ★odd SLEEP: the same trap again, hidden behind a macro call
        SLEEP 4           ; ★even SLEEP: plain NOPs — must NOT fire
        .byte $2c         ; ★same trap again, written as a raw byte (mnemonic matcher is blind)
flag    = $ff             ; var in stack-collision zone...
        jsr Sub           ; ★...which is only a trap because something PUSHES,
        sta flag          ; ★and only if the name is used as an ADDRESS, not as a value
        lda GRP0          ; read of a write-only TIA register
        sta $F123         ; write into cartridge ROM, undeclared
        sta $1F00,x       ; ★indexed store: the phantom read sweeps the whole ROM page
        ; (intentionally no CLD / CLEAN_START)
        org $FFFC         ; ★and this file IS a program: it defines the reset vector, so the
        .word Start       ; ★power-up rules apply to it. An include has no vector of its own.
        .word Start
"""


# --- Self-test counter-bait (nothing here may fire) ---
QUIET = """
        processor 6502
pf1     = $8F             ; ★a RAM variable whose NAME collides with a TIA register, lower case
pf2     = $90             ; ★  (this shape produced 112 false errors in one work's music driver)
COLM1   = $FC             ; ★an EQU in the stack zone that is a VALUE, never an address
Start
        cld
        lda pf1           ; ★reads RAM, not PF1 — the register names are upper case in this tree
        sta PF1           ; writing the register is fine; only reading it is the trap
        lda pf2
        sta PF2
        lda #COLM1        ; ★used with `#`, so it is a colour and cannot be clobbered by a push
        sta COLUP0
        jsr Sub           ; ★there IS a push here, so nothing is passing for want of one
        SLEEP 4           ; even SLEEP is plain NOPs
        rts
Sub     rts
"""

# A third clean sample, the counter-bait the removed `pushes` guard never had: an EQU in the
# stack zone that IS addressed as memory but sits in a file with no push of its own. Rule 3
# must still fire here, because the push lives in whichever file includes this one — which is
# exactly why the file-local push test was unsound.
QUIET_INCLUDE_MUST_FIRE = """
        processor 6502
scratch = $FE
        cld
        sta scratch
"""

# A data-only include: no instruction anywhere, so the CLD rule must not apply to it.
QUIET_TABLE = """
        ; GENERATED by tools/mkpfdata.py -- Do not hand-edit
Table   .byte $00,$01,$03,$07
        .byte $0f,$1f,$3f,$7f
        .word Table
"""

# A CODE include: instructions but no reset vector. Its CLD is in the file that includes it.
# This is what 35 of the author's generated kernel bodies look like.
QUIET_CODE_INCLUDE = """
        ; GENERATED by tools/mk_transistor_sp2x.py. Do not hand-edit.
        ldx #62
SpLead: sta WSYNC
        dex
        bne SpLead
        rts
"""

# A VECTOR-ONLY include: the tail-include idiom, vectors and no code at all.
QUIET_VECTOR_INCLUDE = """
        org $FFFC
        .word Start
        .word Start
"""


def selftest():
    errors, warns = scan_text(BAIT)
    want = ["lxa", "LAX #imm", "bankswitch", "variable at $FF", "no CLD", "WRITE-ONLY TIA register",
            "cartridge ROM", "BIT $2C", "raw byte", "`SLEEP 3` is ODD", "PHANTOM READ"]
    blob = " ".join(m for _, m in errors + warns)
    missing = [w for w in want if w not in blob]
    if missing:
        print("SELFTEST FAIL — detectors didn't fire for:", missing)
        for ln, m in errors + warns:
            print("  got:", m)
        sys.exit(1)
    print("selftest OK — all %d trap detectors fire on the bait" % len(want))

    # ★2026-09-05: and the other half of the control, which this gate did not have. A detector
    # that fires on everything is not a detector; the three false-positive families found when
    # the gate was first aimed at the author's works are each represented here and must stay
    # silent. Aiming a lint at a curated corpus proves it does not miss; only a clean sample
    # proves it does not shout.
    errors, warns = scan_text(QUIET)
    for sample in (QUIET_TABLE, QUIET_CODE_INCLUDE, QUIET_VECTOR_INCLUDE):
        e2, w2 = scan_text(sample)
        errors += e2
        warns += w2
    # ★And the case that must NOT be silenced: a variable genuinely in the stack zone, in a
    # file that pushes nothing itself. An include's caller does the pushing.
    _, w3 = scan_text(QUIET_INCLUDE_MUST_FIRE)
    if not any("variable at $FE" in m for _, m in w3):
        print("SELFTEST FAIL — rule 3 went silent on a real stack-zone variable in an include")
        sys.exit(1)
    if errors or warns:
        print("SELFTEST FAIL — the gate fired on code that is deliberately clean:")
        for ln, m in errors + warns:
            print(f"  line {ln}: {m}")
        sys.exit(1)
    print("selftest OK — and silent on the three false-positive families")


def default_targets():
    """Every .asm this gate is responsible for — WALKED, not listed.

    ★2026-09-05. Until today this was one glob, `roms/techniques/*.asm`, and CI and pre-push both
    called the gate with no arguments — so **31 files** were checked and the author's own works,
    121 + 2 `.asm` between them, were checked by nothing. `docs/gate-ledger.md` recorded this gate
    as "0 catches, kept anyway"; the zero was a statement about where it had been pointed.

    ★★This is the SECOND time the same accident has happened here, and the first one is written
    down in `roms/allworks_test.go`: *"Until 2026-08-15 nothing ran the scenarios in this repo …
    Two directories named 'roms' is the whole of it … Discovery is a WALK, not a list."* That fix
    took 47 scenarios to 151 without a further edit. This one is the same shape and takes the same
    cure. Found by the mailing-list distillation (helper-2), who aimed the gate at the works, then
    opened every one of the 113 findings, and then found the earlier occurrence.

    ★★★The sibling repository may be absent — CI clones only `harness` — so the works are added
    when they are there and their absence is reported rather than assumed. `build/` is excluded
    because it is generated. `sandbox/` is not walked by default: it holds other people's
    disassemblies, which are read only under the clean-room rule, and it is practice rather than
    a deliverable. Pass its paths explicitly to check it.
    """
    out = sorted(glob.glob(os.path.join(HARNESS, "roms", "techniques", "*.asm")))

    works_root = os.path.normpath(os.path.join(HARNESS, "..", "roms"))
    if os.path.isdir(works_root):
        for work in sorted(os.listdir(works_root)):
            src = os.path.join(works_root, work, "src")
            if not os.path.isdir(src):
                continue
            for dirpath, dirnames, filenames in os.walk(src):
                dirnames[:] = [d for d in dirnames if d not in ("build", ".git")]
                out += sorted(os.path.join(dirpath, f)
                              for f in filenames if f.endswith(".asm"))
    else:
        print(f"note: {works_root} is not present, so the works are not covered by this run "
              f"(expected in CI, which clones only this repository)")
    return out


def main():
    if "--selftest" in sys.argv:
        selftest()
        return
    files = [a for a in sys.argv[1:] if not a.startswith("-")]
    if not files:
        files = default_targets()
        if not files:
            print("ERROR: the walk found no .asm file at all — that is a broken walk, not a "
                  "clean tree; a gate that looks at nothing prints the same OK as a gate that "
                  "looks at everything")
            sys.exit(1)
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
