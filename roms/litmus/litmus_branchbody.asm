; litmus_branchbody — a loop body with an if/else in it is still a counted loop.
;
; `loopShape` walks the body header..latch as a STRAIGHT CHAIN and refuses the moment
; it meets a branch:
;
;       if in.Def.IsBranch() {
;               return "branch inside loop body — not a simple counted loop"
;       }
;
; Measured across the sixteen-cartridge corpus, of the branches that trip it:
;
;       89   forward, target still inside the body   (an if/else skip)
;       29   forward, target outside the body        (an early exit)
;        1   backward, inside the body               (an inner loop)
;
; So three quarters of them are a SKIP: the body is not a chain, it is a small
; acyclic graph with two ways through, and both ways come back together before the
; latch. The most common instruction by far is `bcc` (64 of 118) — the shape is
; "add, and if it did not carry, skip the fixup".
;
; A body like that has a longest path, and the longest path is a sound cost for one
; iteration: whichever way the machine goes, it cannot spend more. The refusal was
; about the body being a CHAIN rather than about anything that could not be costed.
;
; THE ROW:
;
;   BranchRow  a counted loop whose body skips two `nop`s on a condition. Both arms
;              rejoin before the latch. Its bound must be the longest arm, and the
;              machine must not exceed it.
;
; THE CONTROLS — each must STAY refused, and each for its own reason:
;
;   ExitCtl    the branch LEAVES the loop. The trip count is then not the counter's
;              range — the loop can end early — and, worse, the cost after the exit
;              belongs to a different part of the region. A longest path through the
;              body says nothing about it.
;   InnerCtl   the branch goes BACKWARD, inside the body: an inner loop. Costing it
;              as one edge of a DAG charges it once for iterations the machine runs
;              many times. This is the shape `overlaps` exists for, and this fixture
;              is what finally reaches that guard.
;   SingleCtl  an ordinary chain body. It must keep the SAME bound, so the repair is
;              measured not to disturb the path every folded region in the corpus
;              takes today.
;
; The predicate reads SWCHB so the abstract interpreter cannot fold an arm away — a
; constant would be pinned and the fixture would exercise a different graph, a mistake
; this repo has made in fixtures twice.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
SWCHB   = $0282

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
; THE ROW THE REPAIR IS FOR: the body skips two `nop`s, and both arms rejoin at the
; decrement. Longest arm = the not-taken one.
BranchRow:
        sta WSYNC
        ldx #4
BrL:    lda SWCHB       ; 4
        and #$02        ; 2
        beq BrSkip      ; 2 not taken / 3 taken
        nop             ; 2   <- the long arm
        nop             ; 2
BrSkip: dex             ; 2
        bne BrL         ; 3 taken / 2 falling out

; CONTROL 1 — the branch leaves the loop. The counter's range is no longer the trip
; count, and the cost after the exit is not part of any iteration.
ExitCtl:
        sta WSYNC
        ldx #4
ExL:    lda SWCHB
        and #$02
        bne ExOut       ; out of the body entirely
        dex
        bne ExL
ExOut:  nop

; CONTROL 2 — an inner loop. Costing this as one DAG edge charges it once for
; iterations the machine runs twice per outer pass.
InnerCtl:
        sta WSYNC
        ldx #3
InOut:  ldy #2
InIn:   dey
        bne InIn
        dex
        bne InOut

; CONTROL 3 — a plain chain body. Its bound must not move.
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
