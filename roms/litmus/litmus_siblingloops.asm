; litmus_siblingloops — two loops on one scanline are not "nested/complex".
;
; `foldLoops` collects the region's backward branches and refuses the whole region
; the moment there is more than one:
;
;       if len(latches) > 1 { return multipleBackEdges }
;
; The refusal is named "multiple back-edges (nested/complex loops)", and the name is
; the mistake. Two loops in a region are not necessarily nested and not necessarily
; complex — the common case is two loops SIDE BY SIDE, each a plain counted loop the
; folder already knows how to handle, one after the other. Measured across the
; sixteen-cartridge corpus, of the regions carrying exactly two latches:
;
;       22   sibling (the intervals do not overlap)
;        9   overlapping (irreducible)
;        1   nested
;
; The refusal's own name describes the rarest of the three. Nesting is rare here for
; a reason that is obvious once measured: a region is one WSYNC-to-WSYNC interval, so
; a nested loop would have to fit two levels of iteration inside 76 cycles.
;
; THE DEFECT IS PRECISION, NOT SOUNDNESS. Refusing is safe; it just answers "I don't
; know" about a region whose cost is a sum of two things it does know. This fixture
; grades the bound against the machine anyway, because a repair that RELAXES a refusal
; is exactly the kind that can introduce an under-approximation — the direction this
; package never allows.
;
; THE ROWS:
;
;   SiblingRow  two counted loops, disjoint intervals, one after the other. The
;               region the repair is for. Its cost is fold(A) + fold(B) + straight
;               line, and both folds are ones the existing code already computes.
;
; THE CONTROLS — both must STAY refused, or the repair has bought precision with
; soundness:
;
;   NestedRow   an inner loop inside an outer loop's body. Folding these
;               independently would charge the inner loop ONCE when the machine runs
;               it once per outer iteration. The body-walk would reject it anyway
;               (the inner latch is a branch inside the outer body), but the point is
;               to measure that, not to assume it.
;   OverlapRow  two loops whose intervals overlap without nesting — B's header sits
;               inside A's body and B's latch sits after A's. Neither is contained in
;               the other, and there is no order in which folding one leaves the other
;               a simple counted loop.
;
;   SingleCtl   one ordinary counted loop. It must keep the SAME bound, so the repair
;               is measured not to disturb the path every folded region in the corpus
;               takes today.
;
; The loops are kept small so all three rows fit their scanlines; the point is the
; SHAPE of the control-flow graph, not the cycle count.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

Main:
; --- VSYNC: 3 lines ---
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 37 lines ---
        lda #2
        sta VBLANK
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

; --- Visible ---
; THE ROW THE REPAIR IS FOR: two counted loops, side by side, disjoint.
SiblingRow:
        sta WSYNC
        ldx #3
SibA:   dex             ; loop A: 3 iterations
        bne SibA
        ldy #4
SibB:   dey             ; loop B: 4 iterations, entirely after A
        bne SibB

; CONTROL 1 — a genuine nest. The inner loop runs once per outer iteration, so
; folding the two independently would charge it once instead of three times.
NestedRow:
        sta WSYNC
        ldx #3
NestOut:
        ldy #2
NestIn: dey
        bne NestIn
        dex
        bne NestOut

; CONTROL 2 — overlapping without nesting: B's header sits INSIDE A's interval and
; B's latch sits OUTSIDE it, so neither interval contains the other and there is no
; order in which folding one leaves the other a simple counted loop.
;
; Both latches are `bpl` on the SAME counter, which is what makes the row terminate.
; The first attempt used `dex`/`dey` with two `bne`s, and the two counters wound each
; other backwards through $FF: the row ran for tens of thousands of cycles, every
; interval crossed a frame boundary, and `ProfileLineWorst` — which drops
; frame-crossing intervals because the coordinate arithmetic does not hold across
; them — returned NOTHING AT ALL for the whole ROM. A fixture that measures nothing
; looks exactly like a fixture that measures agreement.
OverlapRow:
        sta WSYNC
        ldx #3
OvA:    dex             ; A's header. X: 3 -> 2 -> 1 -> 0 -> $FF, then N is set
OvB:    nop             ; B's header, inside A's interval
        bpl OvA         ; A's latch: interval OvA..here
        dex
        bpl OvB         ; B's latch, outside A's interval; N is already set, so 0 trips

; CONTROL 3 — one plain counted loop. Its bound must not move.
SingleCtl:
        sta WSYNC
        ldx #5
SglL:   dex
        bne SglL

        lda #0
        sta COLUBK
        ldx #188
Fill:   sta WSYNC
        dex
        bne Fill

; --- Overscan: 30 lines ---
        sta WSYNC
        lda #2
        sta VBLANK
        ldx #29
OS:     sta WSYNC
        dex
        bne OS

        jmp Main

        org $FFFC
        .word Reset
        .word Reset
