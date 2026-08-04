; litmus_hmove_slope — a 1-pixel object walking an arbitrary-angle straight line,
; because a fractional accumulator decides which scanlines get an HMOVE.
;
; WHAT THE FIXTURE IS FOR. HMOVE moves an object by a WHOLE colour clock or not at
; all, so the only slopes a per-line HMOVE can draw on its own are 0 and ±1 pixels
; per scanline. Every other angle — the fishing line in the Fishing Derby casebook
; (`docs/casebook.md`, capability gap G9(b) in `docs/capability-gap-audit.md`) — has
; to come from an error accumulator: add the fraction every line, and emit a move
; only on the lines that carry. That is Bresenham with HMOVE as the plotter, and
; nothing in this repo measured it.
;
; THE INTENDED LINE EQUATION (this is the claim; the test grades the drawn pixel's
; x against it, not against whatever the harness happened to produce). One byte of
; accumulator, one 8-bit numerator, so the slope is `num/256` pixels per scanline and
; the carry out of `adc` IS the "move this line" flag:
;
;       acc = acc + num          ; carry set  <=>  one whole pixel is owed
;       HMxx = carry ? +-1 : 0   ; consumed by the NEXT line's HMOVE
;
; With the accumulator cleared before the band and the HM register consumed one line
; later, the position of the n-th line of the band is exactly
;
;       x(n) = x(0) + sign * floor(n * num / 256)          n = 0 .. 159
;
; Three 1-pixel objects run the band together, so one frame carries three slopes
; measured against one clock. Colours make them separable in `read_row`: the ball
; takes COLUPF, M0 takes COLUP0, M1 takes COLUP1, the playfield is empty, and no
; player graphic is ever enabled, so each colour appears exactly once per line.
;
;   object  num          direction   slope                       x(0)   x(159)
;   BL      $60 = 96     right       96/256 = 3/8 = 0.375         11      70   (+59)
;   M1      $55 = 85     left        85/256 = 0.33203125         146      94   (-52)
;   M0      (none)       none        HMM0 cleared once, never     86      86   ( +0)
;                                    written again
;
; 3/8 is the worked example the gap was written around: three pixels per eight
; scanlines. 85/256 is deliberately NOT a dyadic fraction, so "arbitrary angle" is not
; a figure of speech — that slope is unreachable by any doubling/halving scheme and
; only an accumulator can draw it.
;
; The three x(0) values are the fixture's only measured inputs, one per object, and
; they are not free either: the positioning kernel is `roms/litmus/litmus_pos.asm`'s
; ÷15 loop verbatim at delay units 2, 7 and 11, whose player positions are 12, 87 and
; 147, and a 1-pixel object inks one clock LEFT of where a player would (measured:
; 11, 86, 146 — the same -1 on all three).
;
; THE ROWS. The 160 lines of the band are the row: every one of them is a reading of
; all three objects, so the fixture's denominator is 160 per object and 480 for the
; frame.
;
; THE CONTROLS — each catches something the slope assertions alone cannot:
;
;   M0 (static)   HMOVE is strobed on all 160 lines, and M0 must not move on ANY of
;                 them. Without it, an engine that shifted every object by a clock
;                 per HMOVE would still satisfy a slope test — it would merely
;                 change what "x(0)" means, and both slopes are graded relative to
;                 their own first line. M0 is the only thing here that says HMOVE
;                 with HM=$00 costs nothing.
;   OPPOSITE      M1 runs leftward while BL runs rightward on the same lines from the
;   DIRECTIONS    same strobe. A sign error in the HM nibble ($10 = left 1 vs $F0 =
;                 right 1) that happened to be symmetric would pass a one-direction
;                 fixture.
;   NON-DYADIC    85/256 is not n/2^k for small k. A regression that quantised the
;   SLOPE         accumulator to a coarser step could still reproduce 3/8.
;   SEPARATION    the three objects' paths are laid out not to cross (BL 11..70,
;                 M0 86, M1 94..146; closest approach 8 px), because TIA priority
;                 would let one hide another and a missing colour reads as "the
;                 object vanished", not as "the object is behind that one".
;
; TIMING. `sta HMOVE` is the first instruction after WSYNC (cycles 0-2, colour clock
; 0-8) so the move belongs to this line and lands deep in HBLANK. The accumulators
; then run, and the HM registers are written at cycles 35 and 38 — past the 24-cycle
; window in which an HMxx write after HMOVE is a known hazard (see the `lint_r3_hazard`
; fixture) and 38 cycles ahead of the next line's strobe. Whole line ~49 cycles.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

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
CTRLPF  = $0A
RESM0   = $12
RESM1   = $13
RESBL   = $14
ENAM0   = $1D
ENAM1   = $1E
ENABL   = $1F
HMM0    = $22
HMM1    = $23
HMBL    = $24
HMOVE   = $2A
HMCLR   = $2B

NUMB    = $60           ; 96/256 = 3/8 px per line, ball, rightward
NUMM    = $55           ; 85/256 px per line, missile 1, leftward

accB    = $80
accM    = $81
lineCnt = $82
posD    = $83

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

        lda #$0E
        sta COLUPF              ; ball  — white
        lda #$46
        sta COLUP0              ; M0    — red
        lda #$B6
        sta COLUP1              ; M1    — green
        lda #0
        sta COLUBK
        sta CTRLPF              ; ball size 1 clock, no PF, no reflect
        sta NUSIZ0              ; missile size 1 clock
        sta NUSIZ1

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

; --- Visible: 192 lines ---
; 1 housekeeping + 3 positioning + 1 arming + 160 slope + 27 tail = 192.
        lda #0
        sta accB
        sta accM
        lda #2                  ; delay unit 2  -> ball inks clock 11
        sta posD
        sta WSYNC               ; ends the housekeeping line

        ; --- positioning: the litmus_pos ÷15 kernel verbatim, one object per line.
        ;     Same instruction sequence and same cycle counts as that ROM, so the
        ;     delay unit -> clock table pinned by TestCoarseAdjustSweepIsWhatTheDocSays
        ;     applies here unchanged. ---
        sta HMCLR
        lda posD
        sec
PD1:    sbc #1
        bcs PD1
        sta RESBL
        lda #7                  ; delay unit 7  -> M0 inks clock 86
        sta posD
        sta WSYNC

        sta HMCLR
        lda posD
        sec
PD2:    sbc #1
        bcs PD2
        sta RESM0
        lda #11                 ; delay unit 11 -> M1 inks clock 146
        sta posD
        sta WSYNC

        sta HMCLR
        lda posD
        sec
PD3:    sbc #1
        bcs PD3
        sta RESM1
        sta WSYNC

        ; --- arming line: enable the three 1-pixel objects, clear the motion
        ;     registers so the band's first HMOVE is a guaranteed no-op ---
        sta HMCLR
        lda #2
        sta ENABL
        sta ENAM0
        sta ENAM1
        lda #160
        sta lineCnt

        ; --- the band: 160 lines, three objects, one strobe ---
SlopeLine:
        sta WSYNC
        sta HMOVE               ; 0-2   this line's move, from last line's carries
        lda accB                ; 3-5
        clc                     ; 6-7
        adc #NUMB               ; 8-9
        sta accB                ; 10-12
        ldx #$00                ; 13-14
        bcc BZ                  ; 15-16
        ldx #$F0                ; 17-18  right 1
BZ:     lda accM                ; 19-21
        clc                     ; 22-23
        adc #NUMM               ; 24-25
        sta accM                ; 26-28
        ldy #$00                ; 29-30
        bcc MZ                  ; 31-32
        ldy #$10                ; 33-34  left 1
MZ:     stx HMBL                ; 35-37  clear of the 24-cycle post-HMOVE hazard
        sty HMM1                ; 38-40
        dec lineCnt             ; 41-45
        bne SlopeLine           ; 46-48

        sta WSYNC               ; ends the band's last line
        lda #0
        sta ENABL
        sta ENAM0
        sta ENAM1
        ldx #27
Tail:   sta WSYNC
        dex
        bne Tail

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS

        jmp Main

        org $FFFC
        .word Reset
        .word Reset
