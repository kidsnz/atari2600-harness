; litmus_midentry — a loop entered PAST its header carries a counter nobody scanned.
;
; `determineBound` takes the counter's entry value by maximising over the predecessors
; of the loop HEADER. That is the right set only if every execution that reaches the
; back edge passed through the header. An edge that lands inside the body — after the
; header, at or before the latch — reaches the latch without ever crossing a scanned
; predecessor, so the value it carries is not in the maximum and the fold charges `n`
; iterations for a loop entered with a different `n`.
;
; Nothing in the analysis stated this premise. The body walk checks that the chain from
; header to latch is straight and cheap; it never asks who else can arrive in it.
;
; Measured on DangerRow before the fix: the header's only scanned predecessor loads
; X=2, while a `jmp` lands mid-body with X=$50 already set. Proven **40** cycles
; against a machine that spends **733 across 10 scanlines** — 18.3x under, reported as
; `certified: true` and `roll_free: true`.
;
; THE SHAPE. `DangerH` is the header (a `nop`); `DangerIn` is one instruction later.
; The big-counter arm skips the header entirely:
;
;       beq DangerSmall   ; the arm the machine does NOT take
;       ldx #$50
;       jmp DangerIn      ; <- lands INSIDE the body, past the header
;   DangerSmall:
;       ldx #$02
;   DangerH:  nop         ; the header, and the only place the scan looks
;   DangerIn: nop
;       dex
;       bne DangerH
;
; The predicate comes from SWCHB so the abstract interpreter cannot fold it away and
; prune one arm — a constant would have been pinned, which is a mistake this repo has
; made twice in fixtures already.
;
; THE CONTROLS:
;
;   JoinCtl   two arms that both enter AT the header, with X in [2,5]. This is the
;             shape a repair keyed on "more than one predecessor" would break, and it
;             is perfectly boundable: the scan sees both and takes the maximum.
;   PlainCtl  a single entry. Must stay bounded and EXACT.
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
; The defect: the machine enters the body past the header with a counter the scan
; never sees. SWCHB's D1 reads high with no switch pressed, so the beq is NOT taken.
DangerRow:
        sta WSYNC
        lda SWCHB
        and #$02
        beq DangerSmall
        ldx #$50        ; the arm the machine takes
        jmp DangerIn    ; lands INSIDE the body
DangerSmall:
        ldx #$02
DangerH:
        nop
DangerIn:
        nop
        dex
        bne DangerH

; Control 1 — two arms, both entering AT the header. Bounded, and the scan's maximum
; is the larger of the two. A repair keyed on "more than one predecessor" loses this.
JoinCtl:
        sta WSYNC
        lda SWCHB
        and #$02
        beq JoinSmall
        ldx #$05
        jmp JoinH
JoinSmall:
        ldx #$02
JoinH:  nop
        nop
        dex
        bne JoinH

; Control 2 — one entry. Must stay bounded and EXACT.
PlainCtl:
        sta WSYNC
        ldx #$03
PlainL: nop
        nop
        dex
        bne PlainL

        lda #0
        sta COLUBK
        ldx #138
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
