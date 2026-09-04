; litmus_resp_pair — what TWO consecutive RESP strobes on one line actually put on the screen.
;
; Manuel Polik, stella-list 200203/msg00074, on placing two sprites with one pass:
;   "STA RESP0 / STA RESP1 in one scannline. With the second RESP, I loose one pixel, so using
;    the same HMOVE values for both sprites would produce a one pixel gap. ... So I worked out
;    two tables, that are shifted by one pixel."
; Read as arithmetic that reads like a contradiction of `sprite-placement.md` rule 1
; (x = 3c - 60): two `sta zp` strobes are 3 cycles apart, so the second lands exactly 9 clocks
; right of the first, on the same 3-clock grid, with no one-pixel remainder anywhere.
;
; It is not a contradiction. Polik is not measuring against the strobe; he is measuring against
; the ADJACENCY he wants. A player is 8 wide, so two players joined edge to edge need +8 and the
; hardware hands him +9 — the pixel he "loses" is the one background pixel left over between the
; two sprites, and his second table is shifted one HMOVE step to spend it. `litmus_p0p1` already
; performs that exact correction (HMP1=$10) and its comment states the +9 it corrects; nothing
; asserted the +9 itself, so the rule and the source were being compared through the HMOVE model
; rather than against the screen.
;
; Four bands, each strobing RESP0 then RESP1 from the identical prelude so the only variable is
; the HMOVE nibble. P0 is white ($0E), P1 is red ($44), background black — so the run boundary in
; `read_row` is the answer and the gap is a run of background between them.
;
;   band  HMP0  HMP1   predicted x0 / x1   what the picture should show
;   A     $00   $00        69 / 78         8w + 1 background + 8r   <- Polik's lost pixel, drawn
;   B     $70   $70        62 / 71         same 9 apart, both moved left 7 (the pair translates)
;   C     $00   $10        69 / 77         16 continuous px, no gap  <- what his second table buys
;   D     $00   $F0        69 / 79         8w + 2 background + 8r    (the gap is a free parameter)
;
; Prelude is copied from litmus_p0p1 on purpose: ldy #8 / dey / bne burns 41 cycles after WSYNC,
; so `sta RESP0` writes on cycle 43 -> x = 3*43-60 = 69, the position that ROM measured.
; A strobe during HBLANK is clamped to the far left, which is why the delay is there.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
NUSIZ1  = $05
COLUP0  = $06
COLUP1  = $07
COLUPF  = $08
COLUBK  = $09
RESP0   = $10
RESP1   = $11
GRP0    = $1B
GRP1    = $1C
HMP0    = $20
HMP1    = $21
HMOVE   = $2A
HMCLR   = $2B

        MAC BAND      ; {1} = HMP0 nibble, {2} = HMP1 nibble
        sta WSYNC           ; setup line: beam at clock 0 of a fresh line
        ldy #8
.dly    dey
        bne .dly            ; 41 cycles burned -> out of HBLANK
        sta RESP0           ; write cycle 43 -> x0 = 69
        sta RESP1           ; write cycle 46 -> x1 = 78   (+9, the whole question)
        lda #{1}
        sta HMP0
        lda #{2}
        sta HMP1
        sta WSYNC           ; HMOVE line (carries the comb; not a measured line)
        sta HMOVE
        lda #$FF
        sta GRP0
        sta GRP1
        ldx #8
.drw    sta WSYNC
        dex
        bne .drw            ; 8 drawn lines
        lda #0
        sta GRP0
        sta GRP1
        sta HMCLR
        ENDM

        MAC BLANKS    ; {1} = how many
        ldx #{1}
.blk    sta WSYNC
        dex
        bne .blk
        ENDM

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
        lda #$0E
        sta COLUP0          ; P0 white
        lda #$44
        sta COLUP1          ; P1 red — the two are told apart by colour, not by guessing
        lda #0
        sta NUSIZ0
        sta NUSIZ1
        sta COLUBK
        sta GRP0
        sta GRP1

NextFrame:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
Vb:     BLANKS 37           ; 37 VBLANK lines -> 3+37 = 40 before the picture
        lda #0
        sta VBLANK

Lead:   BLANKS 8            ; lead-in
BandA:  BAND $00,$00
GapA:   BLANKS 4
BandB:  BAND $70,$70
GapB:   BLANKS 4
BandC:  BAND $00,$10
GapC:   BLANKS 4
BandD:  BAND $00,$F0
Tail:   BLANKS 132          ; 8+10+4+10+4+10+4+10+132 = 192

        lda #2
        sta VBLANK
Over:   BLANKS 30
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
