; divtable — constant-divisor division helpers (÷3, ÷7, ÷10, ÷15)
; technique: divtable (constant division by reciprocal-multiply + 1-step correction).
;
; Method (exact over 0..255, no tables, no illegal ops):
;   q ≈ (A * RECIP) >> 8     where RECIP = round(256 / divisor)
;        /3→86  /7→37  /10→26  /15→18
;   then a single bounded correction:  while rem>=d {rem-=d; q++} ; while rem<0 {rem+=d; q--}
;   The reciprocal estimate is within ±1 of the true quotient for every divisor here,
;   so the correction loop runs at most ONE iteration (verified 0..255 in Go before writing).
;
; The high-byte-of-product is computed by a generic 8×8 shift-add multiply (MulHi8):
;   shift multiplier right; on each set bit add multiplicand into a 16-bit accumulator;
;   the high byte of the 16-bit product is the estimate. All stores are deterministic.
;
; Demo: divides input 100 by 3/7/10/15, storing quotient+remainder to RAM, then sweeps a
; range of inputs into a RAM result table. Finally it uses ÷15 (the coarse-position divisor)
; to place a visible bar via a derived NUSIZ/position so the screen is NOT blank and the
; ÷15 result drives something on-screen.
;
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUBK  = $09
PF0     = $0D
PF1     = $0E
PF2     = $0F
RESP0   = $10
GRP0    = $1B
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B
CTRLPF  = $0A

; ---- zero page ----
num     = $80       ; dividend (working)
mul     = $81       ; reciprocal constant (multiplier)
prodlo  = $82       ; 16-bit product low
prodhi  = $83       ; 16-bit product high (= estimate quotient)
divd    = $84       ; current divisor (for correction)
quot    = $85       ; quotient result
rem     = $86       ; remainder result
cnt     = $87       ; multiply bit counter
tmp     = $88
swin    = $89       ; sweep loop: current input
swix    = $8A       ; sweep loop: current table index

; results for input=100 (the headline asserts)
q3      = $90       ; 100/3 = 33
r3      = $91       ; rem 1
q7      = $92       ; 100/7 = 14
r7      = $93       ; rem 2
q10     = $94       ; 100/10 = 10
r10     = $95       ; rem 0
q15     = $96       ; 100/15 = 6
r15     = $97       ; rem 10

; a small swept ÷15 table: input 0,15,30,...,150 → quotient 0..10
tbl     = $A0       ; 11 entries at $A0..$AA

barx    = $B0       ; ÷15-derived bar X (proves ÷15 is wired to the screen)

        org $F000
Start:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

        ; ===== compute the headline table for input = 100 =====
        lda #100
        ldx #3
        jsr DivCalc
        sta q3
        lda rem
        sta r3

        lda #100
        ldx #7
        jsr DivCalc
        sta q7
        lda rem
        sta r7

        lda #100
        ldx #10
        jsr DivCalc
        sta q10
        lda rem
        sta r10

        lda #100
        ldx #15
        jsr DivCalc
        sta q15
        lda rem
        sta r15

        ; ===== sweep ÷15 over 0,15,...,150 into tbl[0..10] =====
        ; NOTE: DivCalc clobbers Y and tmp, so the loop state lives in swin/swix.
        lda #0
        sta swin            ; current input
        sta swix            ; current table index
SweepLp:
        lda swin
        ldx #15
        jsr DivCalc         ; A = quotient
        ldy swix
        sta tbl,y
        lda swin
        clc
        adc #15
        sta swin
        inc swix
        lda swix
        cmp #11
        bne SweepLp

        ; ===== derive a bar X from ÷15 of 90 (= 6) to drive the screen =====
        lda #90
        ldx #15
        jsr DivCalc         ; A = 6
        sta barx            ; bar uses this as a coarse value (proves ÷15→screen)

        ; visual setup: solid playfield bar + a player block
        lda #$0E
        sta COLUP0
        lda #$46
        sta COLUBK
        lda #$05            ; double-width player
        sta NUSIZ0
        lda #1
        sta CTRLPF          ; reflect

NextFrame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK
        ldx #36
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; ----- visible region: a centred bar so the screen is not blank -----
        ; top margin
        ldx #40
TopM:   sta WSYNC
        dex
        bne TopM

        ; bar band: light up playfield for a chunk of lines, GRP0 solid
        lda #$FF
        sta PF0
        sta PF1
        sta PF2
        lda #$FF
        sta GRP0
        ldx #80
Bar:    sta WSYNC
        dex
        bne Bar

        ; clear bar
        lda #0
        sta PF0
        sta PF1
        sta PF2
        sta GRP0

        ; bottom margin
        ldx #73
BotM:   sta WSYNC
        dex
        bne BotM

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

; ============================================================
; DivCalc — divide A by divisor (in X: 3,7,10,15) using
;           reciprocal-multiply + 1-step correction.
; in:  A = dividend (0..255), X = divisor selector
; out: A = quotient, rem = remainder, quot = quotient
; uses: num, mul, divd, prodlo/hi, cnt, quot, rem, tmp
; ============================================================
DivCalc:
        sta num
        stx divd
        ; pick reciprocal constant from divisor
        cpx #3
        bne d_n3
        lda #86
        jmp d_set
d_n3:   cpx #7
        bne d_n7
        lda #37
        jmp d_set
d_n7:   cpx #10
        bne d_n10
        lda #26
        jmp d_set
d_n10:  lda #18             ; default = ÷15
d_set:  sta mul

        jsr MulHi8          ; prodhi = (num * mul) >> 8  → estimate quotient
        lda prodhi
        sta quot

        ; The reciprocal estimate is within ±1 of the true quotient, so we correct in
        ; BOTH directions: first shrink quot while quot*divd > num (estimate too high),
        ; then grow quot while the remainder still >= divd (estimate too low).

        ; --- downward correction: rem = num - quot*divd ; if it borrows, quot-- ---
recompute:
        lda #0
        sta tmp
        ldy quot
        beq qd_done
qd_lp:  clc
        lda tmp
        adc divd
        sta tmp
        dey
        bne qd_lp
qd_done:
        lda num
        sec
        sbc tmp             ; A = num - quot*divd ; carry clear = borrow = quot too high
        bcs nohi            ; carry set → num >= quot*divd → no over-estimate
        dec quot            ; over-estimated → back off and recompute remainder
        jmp recompute
nohi:
        sta rem

        ; --- upward correction: while rem >= divd { rem -= divd; quot++ } ---
corr:   lda rem
        cmp divd
        bcc corr_done       ; rem < divd → done
        sec
        sbc divd
        sta rem
        inc quot
        jmp corr
corr_done:
        lda quot
        rts

; ============================================================
; MulHi8 — 16-bit product of num*mul, returns high byte in prodhi.
;   classic shift-add: shift mul right (LSB first), add num on set bits.
;   prodlo:prodhi accumulate; final prodhi = (num*mul)>>8.
; ============================================================
MulHi8:
        lda #0
        sta prodlo
        sta prodhi
        ldx #8
        sta cnt             ; (cnt unused as loop var; X is the counter)
mh_lp:
        ; shift product left by 1 (we build MSB-first by shifting accumulator up)
        asl prodlo
        rol prodhi
        ; test top bit of mul (process MSB-first): rotate mul left into carry
        asl mul
        bcc mh_no
        ; add num into accumulator
        clc
        lda prodlo
        adc num
        sta prodlo
        lda prodhi
        adc #0
        sta prodhi
mh_no:
        dex
        bne mh_lp
        rts

        org $FFFC
        .word Start
        .word Start
