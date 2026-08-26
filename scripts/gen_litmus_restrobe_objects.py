#!/usr/bin/env python3
"""Build roms/litmus/litmus_restrobe_objects.asm/.bin — which OBJECTS a mid-line strobe re-draws.

Graded by internal/emu/restrobeobjects_test.go; the rules land in docs/techniques/sprite-placement.md
(rules 11 and 13) and docs/techniques/restrobe-copies.md.

WHY IT EXISTS. sprite-placement.md rule 11 says "the BALL places exactly like a missile: same
x = 3c - 61, same clamp at 2". That is true and it is about PLACEMENT, but it invites a second
inference — that the two also behave alike when strobed again while they are on screen — and three
throwaway probes in a private work measured exactly that, in ONE state each (M0 only, one park, one
width, one spacing). A ladder measured in one state cannot tell a law from a coincidence; that is
the rule check_instruments.py enforces for measurement functions and the reason litmus_restrobe was
rewritten. So this fixture sweeps the state instead of asserting from one point of it.

WHAT IS SWEPT.

  part A -- the ladder: does a second strobe ADD a block, or move the one that is there?
            object (M0, M1, BL, and P0 with ONE copy as the control) x width (2 px, 8 px)
            x strobe spacing (3, 8, 16 cycles) x k (1, 2, and 3 at one point of the grid).
            The strobes run at cycle 34 and later, after the parked block has finished drawing,
            so this asks about adding a block and NOT about interrupting one.

  part B -- the same strobe aimed INSIDE the block that is drawing: park at x=50, strobe once so
            the new base lands at +6, +9 or +12 px. (+15 was dropped: it would fit the
            picture only by giving up a ladder band, and +12 already shows a clean gap.)
            The ball's grid is 3 px, so +8 -- what
            butting an 8 px block against another would need -- is not a position it can take;
            +6 and +9 are the two nearest offers the hardware makes.

HOW A BAND IS READ. Three lines each: a re-park line whose strobe may draw a partial block and is
therefore NOT a baseline, an untouched SENTINEL line showing the parked block alone, and the
measurement line. The grader anchors by finding the one offset at which EVERY band's sentinel
matches that band's own element, x and width — a wrong anchor fails on the next band rather than
grading the wrong rows quietly, which is how litmus_restrobe's first grader went wrong.
"""
import os
import subprocess

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)

PARK_CYCLE = 26          # setup fills cycles 0..23; the park store then occupies 24-26
PARK_X = 3 * PARK_CYCLE - 61          # 20 for missile/ball
PARK_X_P = 3 * PARK_CYCLE - 60        # 21 for a player: rule 1 vs rule 2, one clock apart
FIRST_A = 34             # first ladder strobe: the parked 8 px block ends at x=27, beam cycle 32
LAST_CYCLE = 74
MID_X = 50               # part B parks here so the strobe lands while the block is drawing
MID_CYCLE = (MID_X + 61) // 3
OFFSETS = (6, 9, 12, 15)
SPACINGS = (3, 8, 16)
WIDTHS = (1, 3)          # NUSIZ/CTRLPF size field: 1 = 2 px, 3 = 8 px
PIX = {0: 1, 1: 2, 2: 4, 3: 8}

# object -> (element, strobe reg, enable reg, enable value, width reg, dark value)
OBJ = {
    "M0": ("M0", "RESM0", "ENAM0", "$02", "NUSIZ0", "$00"),
    "M1": ("M1", "RESM1", "ENAM1", "$02", "NUSIZ1", "$00"),
    "BL": ("BL", "RESBL", "ENABL", "$02", "CTRLPF", "$00"),
    "P0": ("P0", "RESP0", "GRP0", "$FF", "NUSIZ0", "$00"),
}


def pad(n):
    """n cycles of nothing. nop is 2 and bit on zero page is 3, so one cycle cannot be built."""
    if n < 0 or n == 1:
        raise SystemExit("pad %d cannot be built" % n)
    out = []
    if n % 2:
        out.append("        bit $80")
        n -= 3
    return out + ["        nop"] * (n // 2)


def setup(obj, size):
    """Cycles 1..24 of a re-park line: everything dark, then this band's object armed and sized.

    The length is FIXED so that every band parks on the same cycle and therefore the same x —
    a sentinel that moved with the band would have to be described twice, in the fixture and in
    the grader, and the two would drift.
    """
    el, res, enr, env, wr, _ = OBJ[obj]
    wval = "$%02X" % (size << 4) if obj != "P0" else "$00"
    return ([                              # cycles 1..14: nothing is on
        "        lda #$00",
        "        sta ENAM0", "        sta ENAM1", "        sta ENABL", "        sta GRP0",
    ] + [                                  # 15..19: this band's width
        "        lda #%s" % wval, "        sta %-6s" % wr,
    ] + [                                  # 20..24: arm it
        "        lda #%s" % env, "        sta %-6s" % enr,
    ])


def strobes(cycles, reg, start=0):
    out, cy = [], start
    for c in cycles:
        out += pad(c - 2 - cy)
        out.append("        sta %-6s      ; write cycle %d -> base x=%d" % (reg, c, 3 * c - 61))
        cy = c + 1
    return out


def buildable(cycles):
    cy = 0
    for c in cycles:
        gap = c - 2 - cy
        if gap < 0 or gap == 1:
            return False
        cy = c + 1
    return cycles[-1] <= LAST_CYCLE


# ---------------------------------------------------------------- the band list
# (tag, obj, size, kind, payload). Order is the contract with the grader.
# P0 HAS NO WIDTH AXIS. Its width is GRP0's eight bits, not a size field, so sweeping WIDTHS on it
# would produce six pairs of identical bands with a tag that says otherwise. It gets the spacings
# and the k ladder only, and the six lines that buys pay for part B's +15 offset and for asking
# part B's question of a player as well.
BANDS = []
for obj in ("M0", "M1", "BL", "P0"):
    sizes = WIDTHS if obj != "P0" else (0,)
    for size in sizes:
        for s in SPACINGS:
            for k in (1, 2):
                cs = [FIRST_A + i * s for i in range(k)]
                if buildable(cs):
                    w = "w%d " % PIX[size] if obj != "P0" else ""
                    BANDS.append(("%s %ss%d k%d" % (obj, w, s, k), obj, size, "A", cs))
    # k=3 at one point of the grid: two points make a slope, three make it a ladder
    cs = [FIRST_A + i * 8 for i in range(3)]
    BANDS.append(("%s %ss8 k3" % (obj, "w8 " if obj != "P0" else ""),
                  obj, 3 if obj != "P0" else 0, "A", cs))
for obj in ("M0", "M1", "BL", "P0"):
    for off in OFFSETS:
        # the player's grid is 3c-60 and the others' is 3c-61, so the same store cycle reaches a
        # base one clock further right; the offset is the same because both grids step by three.
        BANDS.append(("%s %smid+%d" % (obj, "w8 " if obj != "P0" else "", off),
                      obj, 3 if obj != "P0" else 0, "B", [(MID_X + off + 61) // 3]))

L = ["; litmus_restrobe_objects.asm — GENERATED by scripts/gen_litmus_restrobe_objects.py.",
     "; Do not hand-edit. Three lines per band: re-park (not a baseline), sentinel, measurement.",
     "        processor 6502",
     "VSYNC = $00", "VBLANK = $01", "WSYNC = $02", "NUSIZ0 = $04", "NUSIZ1 = $05",
     "COLUP0 = $06", "COLUP1 = $07", "COLUPF = $08", "COLUBK = $09", "CTRLPF = $0A",
     "RESP0 = $10", "RESM0 = $12", "RESM1 = $13", "RESBL = $14",
     "GRP0 = $1B", "GRP1 = $1C", "ENAM0 = $1D", "ENAM1 = $1E", "ENABL = $1F",
     "        org $F000",
     "Reset:  sei", "        cld", "        ldx #0", "        txa",
     "Clear:  dex", "        txs", "        pha", "        bne Clear",
     "        lda #$00", "        sta COLUBK", "        sta GRP0", "        sta GRP1",
     "        sta ENAM0", "        sta ENAM1", "        sta ENABL",
     "        sta NUSIZ0", "        sta NUSIZ1", "        sta CTRLPF",
     "        lda #$0E", "        sta COLUP0", "        sta COLUPF",
     "        lda #$44", "        sta COLUP1",
     "Frame:  lda #2", "        sta VSYNC",
     "        sta WSYNC", "        sta WSYNC", "        sta WSYNC",
     "        lda #0", "        sta VSYNC",
     "        ldx #36", "VbPad:  sta WSYNC", "        dex", "        bne VbPad",
     "        sta WSYNC", "        lda #0", "        sta VBLANK"]

lines = 0
for tag, obj, size, kind, payload in BANDS:
    el, res, enr, env, wr, _ = OBJ[obj]
    park = PARK_CYCLE if kind == "A" else MID_CYCLE
    x = (3 * park - 60) if obj == "P0" else (3 * park - 61)
    L.append("; ---- band %s (%s): park x=%d, %s ----"
             % (tag, kind, x, "strobes at %s" % payload if kind == "A"
                else "one strobe at cycle %d -> x=%d" % (payload[0], 3 * payload[0] - 61)))
    L.append("        sta WSYNC")
    L += setup(obj, size)
    L += pad(park - 2 - 24)
    L.append("        sta %-6s      ; park: write cycle %d -> x=%d" % (res, park, x))
    lines += 1
    L.append("        sta WSYNC         ; sentinel: the parked block alone, %s at x=%d width %d"
             % (el, x, 1 if obj == "P0" else PIX[size]))
    lines += 1
    L.append("        sta WSYNC")
    L += strobes(payload, res)
    lines += 1

rest = 192 - lines
if rest < 1:
    raise SystemExit("picture is %d lines, over the 192 budget" % lines)
L += ["        lda #$00", "        sta ENAM0", "        sta ENAM1", "        sta ENABL",
      "        sta GRP0",
      "        ldx #%d" % rest, "Rest:   sta WSYNC", "        dex", "        bne Rest",
      "        lda #2", "        sta VBLANK",
      "        ldx #30", "OsPad:  sta WSYNC", "        dex", "        bne OsPad",
      "        jmp Frame",
      "        org $FFFC", "        word Reset", "        word Reset"]

asm = os.path.join(ROOT, "roms", "litmus", "litmus_restrobe_objects.asm")
binp = os.path.join(ROOT, "roms", "litmus", "litmus_restrobe_objects.bin")
open(asm, "w").write("\n".join(L) + "\n")
subprocess.run(["dasm", asm, "-f3", "-o" + binp], check=True)
print("%d bands, %d picture lines, %d spare" % (len(BANDS), lines, rest))
print("part A (ladder): %d   part B (strobe inside the drawing block): %d"
      % (sum(1 for b in BANDS if b[3] == "A"), sum(1 for b in BANDS if b[3] == "B")))
for tag, obj, size, kind, payload in BANDS:
    print("  %-16s %s" % (tag, payload))
