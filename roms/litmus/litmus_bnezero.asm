; litmus_bnezero — a `dex; bne` counter entering at ZERO runs 256 times, not none.
;
; `determineBound` takes the counter's entry range from the header's predecessors and
; returns its upper bound `Hi`, on the reasoning that more iterations cost more so the
; largest entry value is the worst case. For `dex; bne` that reasoning is wrong at
; exactly one point: the trip count as a function of the entry value v is
;
;       v      for v > 0     (the decrement reaches zero after v steps)
;       256    for v = 0     (the decrement wraps to $FF and counts down from there)
;
; which is not monotone. `Hi` is therefore not the maximum whenever 0 is in the range,
; and the analysis answers for the SMALLEST possible loop while the machine runs the
; largest one.
;
; Measured on DangerRow before the fix: the entry range at the header is [0,5], the
; prover answers n=5 and 67 cycles, and the machine spends 2326 across 31 scanlines —
; **34.7x under**, reported as `certified: true` and `roll_free: true`.
;
; This was found, censused and deliberately left alone when the sibling `bpl` bug was
; fixed (see the SD-11 note in determineBound): 30 bne folds across 140 images, 3 with
; a range including 0, none violating. Re-censused across all 155 images once the gate
; stopped grading a hand-picked five, it is **14 folds in 3 commercial cartridges** —
; Seaquest x3, Bermuda Triangle x6, Vanguard x5, all with range [0,15]. Latent in every
; observed run, and one input away from live in three shipped games.
;
; THE RANGE IS BUILT BY A JOIN, not by a constant. `lda SWCHB / and #$02 / beq` gives
; the abstract interpreter two paths into the header — one with X=0, one with X=5 — so
; the state there is genuinely [0,5]. A fixture that just wrote `ldx #0` would be
; pinned to the single value and would exercise a different branch of the analysis;
; that mistake has been made twice in this repo already.
;
; THE CONTROLS, each ruling out a different wrong repair:
;
;   PosCtl    the same join with X in [3,5] — 0 is NOT in the range, so this loop is
;             exactly as boundable as before. A repair that refuses every BNE, or every
;             joined range, loses it.
;   ConstCtl  a plain `ldx #5` countdown, range [5,5]. Must stay EXACT: the fix must
;             key on 0 being reachable, not on the latch being BNE.
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
; The defect. The join gives the header X in [0,5]; the machine takes the X=0 arm
; (SWCHB's D1 reads high with no switch pressed, so the beq is NOT taken) and the
; counter wraps for 256 iterations. Must be bounded at the WRAP count, or refused.
DangerRow:
        sta WSYNC
        lda SWCHB
        and #$02
        beq DangerBig
        ldx #$00        ; the arm the machine takes
        jmp DangerL
DangerBig:
        ldx #$05
DangerL:
        nop
        nop
        dex
        bne DangerL

; Control 1 — the same shape with 0 out of the range. Must stay bounded and EXACT.
PosCtl: sta WSYNC
        lda SWCHB
        and #$02
        beq PosBig
        ldx #$03
        jmp PosL
PosBig: ldx #$05
PosL:   nop
        nop
        dex
        bne PosL

; Control 2 — a plain constant countdown. Must stay bounded and EXACT: the repair has
; to key on zero being reachable, not on the latch being a BNE.
ConstCtl:
        sta WSYNC
        ldx #$05
ConstL: nop
        nop
        dex
        bne ConstL

        lda #0
        sta COLUBK
        ldx #140
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
