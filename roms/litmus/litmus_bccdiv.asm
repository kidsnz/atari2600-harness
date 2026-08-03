; litmus_bccdiv — BCC counts UP, so the BCS formula uses the wrong variable.
;
; `determineBound` bounds the `sbc #N` divide idiom with `amax/sub + 2` and applies it
; to BOTH latches. That is right for BCS and wrong for BCC, because the two loops move
; in opposite directions:
;
;   `sbc #N / bcs L`  loops while NO borrow, i.e. while A >= N. A falls by N each time,
;                     so a LARGER entry value means MORE iterations and `amax` is the
;                     variable that bounds it.
;
;   `sbc #N / bcc L`  loops while there IS a borrow, i.e. while A < N. The subtraction
;                     wraps, so A RISES by (255 - N) each time until it reaches N. A
;                     larger entry value means FEWER iterations, and `amax` bounds
;                     nothing at all — the count depends on N alone.
;
; The worst case is therefore A = 0, and the bound is `ceil(N / (255 - N)) + 2`, not
; `amax / N + 2`. The two agree only while N is small: at N = 15 the old formula is
; loose and safe, at N = 200 it answers 2 for a loop the machine runs 5 times.
;
; Measured on DangerRow before the fix: proven **16** cycles against a machine that
; spends **31** — 1.9x under, with `certified: true`. Small in absolute terms and the
; smallest of the nine unsound bounds the premise audit found, but the same direction
; as all of them, and it is the last one.
;
; N = 255 does not terminate at all: 255 - 255 = 0, so A never rises. Refused.
;
; THE CONTROLS:
;
;   BcsCtl   the BCS form with the same subtrahend. Its bound is the one `amax` really
;            does govern, and it must not move — the fix must key on the LATCH, not on
;            the divide idiom.
;   SmallCtl a BCC with N = 15, where the old formula was already safe. Must stay
;            bounded, so the repair is measured not to refuse the common case.
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
; The defect: A enters at 0 and rises by 55 each wrap, taking 5 iterations to reach
; 200. The BCS formula answers 0/200 + 2 = 2.
DangerRow:
        sta WSYNC
        lda #$00
        sec
BccL:   sbc #$C8        ; 200
        bcc BccL

; Control 1 — the BCS form, where amax is genuinely the governing variable.
BcsCtl: sta WSYNC
        lda #$3C        ; 60
        sec
BcsL:   sbc #$0F        ; 15
        bcs BcsL

; Control 2 — a BCC with a small subtrahend, where the old formula was already safe.
SmallCtl:
        sta WSYNC
        lda #$00
        sec
SmallL: sbc #$0F        ; 15 — A rises by 240 a step, so one wrap clears it
        bcc SmallL

        lda #0
        sta COLUBK
        ldx #142
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
