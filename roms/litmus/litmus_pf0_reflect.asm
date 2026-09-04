; litmus_pf0_reflect.asm — writing PF0 twice in one line, under reflection, to show
; different values at the left and right edges.
;
; STAGE 1 of 2: the calibration band and the RIGHT-edge sweep only. The right edge is
; where this fixture can break, so it is built and measured before the rest is added.
;
; What is being measured, and what is not:
;
;   measurable  "one line, two PF0 writes, different values at the two edges" —
;               does the technique work at all, and where are its boundaries
;   NOT ours    "real games do this" (DaveC's _room_loop) — someone else's source
;
; Under reflection PF0 draws twice: cols 0-3 at the left and cols 36-39 at the right.
; design-principles gives 68 colour clocks of HBLANK = 22 2/3 CPU cycles, so the left
; copy is drawn around cy 22.7-27.7 and the right copy around cy 70.7-75.7. A second
; write landing between them changes the right edge and not the left, which is the
; whole trick — and it means the sweep must cross ONE of those two windows. Sweeping
; cy 40..55, as the first design said, sweeps the blind gap between them and finds no
; step at all. That is what `0 < steppos < last` below is for: a sweep that lands on an
; end is a sweep in the wrong place, and should say so rather than read as a null result.
;
; THE MEASURING INSTRUMENT IS BUILT HERE FIRST. Reading a collision field as a sensor —
; "is P1 over the playfield right now?" — needs it to read 0 when they are apart, and
; nothing in this repo measures that: litmus_collide_all overlaps everything with
; everything and its scenario asserts fifteen 1s and no 0s. So band 0 establishes both
; directions for the two pairs this ROM uses, and the sweep below rests on that.
;
; EVERY POINT TAKES EXACTLY TWO LINES. A swept store that completes late leaves the
; loop control running past the end of the line, so an iteration would cost one line
; or two depending on the sweep point, and the frame length would change mid-band.
; litmus_flicker_attrib was rebuilt three times for the same reason. So: line 1 sets
; up and ends with the swept store, line 2 does all the reading and counting with a
; full 76 cycles to do it in. Nothing crosses a line boundary.
;
; Self-contained (no vcs.h). Frame length is asserted by frame_lines_stable, not
; derived: see known-traps section A.
; Design by the mailing-list distillation (helper-3), 2026-09-03.

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
PF0     = $0D
PF1     = $0E
PF2     = $0F
RESP0   = $10
RESP1   = $11
HMP1    = $21
HMOVE   = $2A
HMCLR   = $2B
GRP0    = $1B
GRP1    = $1C
CXCLR   = $2C
CXP0FB  = $02          ; read: D7 = P0/PF
CXP1FB  = $03          ; read: D7 = P1/PF

cur     = $80          ; this point's normalised reading
prev    = $81          ; the previous point's, for the step count
idx     = $82          ; point index inside the band
stepn   = $83          ; steps seen in this band
steppos = $84          ; index of the last step

; band 0 — the instrument, both directions, both pairs
c0lit   = $88          ; P0 left,  PF0 lit    -> want 1
c0drk   = $89          ; P0 left,  PF0 blank  -> want 0
c1lit   = $8A          ; P1 right, PF0 lit    -> want 1
c1drk   = $8B          ; P1 right, PF0 blank  -> want 0

cEF0    = $8C          ; E: left lit, right blank
cEF1    = $8D          ; F: left blank, right lit
scratch = $8E

arSteps = $92          ; band A-R: number of steps    -> want 1
arPos   = $93          ; band A-R: where the step is  -> want strictly inside

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
        sta CXCLR

        lda #$0E
        sta COLUP0
        sta COLUP1
        sta COLUPF
        lda #$00
        sta COLUBK
        sta NUSIZ0
        sta NUSIZ1      ; P1 stays ONE COPY, NORMAL WIDTH — 8 px.
                        ; A first version made it quad width (32 px) to be sure of reaching
                        ; PF0's mirrored copy at pixels 144-159. It reached it, and also
                        ; reached the copy at the other end: the TIA's horizontal counter
                        ; wraps at 160, so a 32-px object starting at 151 draws 151-159 and
                        ; then 0-22 of the SAME line, straight across PF0's left copy at
                        ; 0-15. The probe was standing on both copies at once, which is why
                        ; points E and F below both read 1 and why the first sweep tracked
                        ; the LEFT copy's boundary at cy 27.7 while claiming to watch the
                        ; right one. Width is not free at the right-hand edge.
        lda #$FF
        sta GRP0
        sta GRP1
        lda #$01
        sta CTRLPF      ; REFLECT — PF0 draws at cols 0-3 and again at 36-39
        lda #$00
        sta PF1
        sta PF2         ; PF1/PF2 stay dark: only PF0 is under test

Main:
; --- VSYNC: 3 lines ---
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: place P0 at the left edge and P1 at the right edge ---
        lda #2
        sta VBLANK
        sta WSYNC

        ldx #33
VB:     sta WSYNC
        dex
        bne VB

        sta WSYNC
        lda #12         ; P0: left edge
        sec
D0:     sbc #15
        bcs D0
        sta RESP0

        sta WSYNC
        lda #193        ; P1: as far right as one line of div-15 reaches (~px 139)
        sec
D1:     sbc #15
        bcs D1
        sta RESP1
        ; Fine-adjust P1 seven pixels LEFT, to the leading edge of PF0's mirrored copy.
        ; The div-15 idiom quantises to 15 px a turn, so it cannot place a probe inside a
        ; 16-px band; HMOVE is what the hardware provides for the remainder. It matters
        ; here because the sweep's last useful point lands INSIDE the copy, splitting it
        ; old|new, and a probe sitting at the copy's trailing edge sees only the new half.
        lda #$70                ; +7 = seven pixels left
        sta HMP1
        sta WSYNC
        sta HMOVE
        sta WSYNC               ; HMCLR goes on the NEXT line. Written two cycles after the
                                ; HMOVE it killed the move outright and P1 never left 151.
                                ; Note what that does and does not say: litmus_hmxx_freeze
                                ; strobes HMCLR about fifty cycles after its own HMOVE, on
                                ; the same line, and its scenario pins the player moving +8
                                ; a frame regardless — so "same line" is not the rule. The
                                ; likely one is already in fundamentals-audit: do not write
                                ; HMxx within 24 cycles of HMOVE, with HMCLR counting as an
                                ; HMxx write. Unseparated here: we changed the distance AND
                                ; the kind of write at once.
        sta HMCLR               ; one adjustment, not one per line
        lda #0
        sta VBLANK
        sta WSYNC

; ===========================================================================
; Visible: 192 lines.  band 0 = 8, band A-R = 32, the rest dark.
; ===========================================================================

; --- band 0: the instrument. Four points, two lines each. ---
        lda #$F0
        sta PF0
        jsr Read0               ; P0 left, lit
        sta c0lit
        lda #$00
        sta PF0
        jsr Read0               ; P0 left, blank
        sta c0drk
        lda #$F0
        sta PF0
        jsr Read1               ; P1 right, lit
        sta c1lit
        lda #$00
        sta PF0
        jsr Read1               ; P1 right, blank
        sta c1drk

        ; Points E and F: which copy is the probe actually reading? The four points
        ; above cannot say. They set PF0 for the whole line, so the left copy and the
        ; right copy are always in the SAME state and a probe anywhere on the playfield
        ; returns 1,0,1,0 — including a probe sitting on the wrong copy. That is the
        ; failure this whole week has been about, in the instrument built to avoid it.
        ;
        ; E and F make the two copies DISAGREE, by writing the second value in the blind
        ; gap between them (cy ~50, after the left copy at 22.7-27.7 and before the right
        ; at 70.7-75.7). The gap that made the first sweep design useless is what makes
        ; this discrimination possible.
        ;
        ;   (E,F) = (0,1)  reading the RIGHT copy   (E,F) = (1,0)  reading the LEFT copy
        ;   (E,F) = (1,1)  covering both            (E,F) = (0,0)  covering neither
        jsr Split1              ; E: lit early, blanked in the gap
        sta cEF0
        jsr Split0              ; F: blank early, lit in the gap
        sta cEF1

; --- band A-R: sweep the second write across the RIGHT copy (cy ~70.7-75.7) ---
        lda #0
        sta idx
        sta stepn
        sta steppos
        sta prev
        ldy #7
ARLoop:
        ; ---- line 1: set up, then the swept store ends the line ----
        ;
        ; The sweep has to CROSS the right copy at cy 70.7-75.7, and there is no room
        ; to finish after it: the line ends at 76. So the last point lands INSIDE the
        ; copy and the reading changes from "all new" to "part old" — a split, which a
        ; two-valued collision probe still reports as 1. Seven points, cy 41 to 71.
        ;
        ; TBASE (no padding) is ~19. The fixed pad below moves the start to 41; the
        ; variable pad after it adds 5 cycles a point. Aiming the start is what the
        ; first version lacked — it swept from wherever the instructions happened to
        ; end, cy 19 to 93, across both copies and past the end of the line.
        sta WSYNC
        sta CXCLR               ; clear before the line is drawn
        lda #$F0
        sta PF0                 ; first write: lit, before the left copy at 22.7
        ldx #3
ARFix:  dex                     ; fixed pad: 3 turns = 14 cy
        bne ARFix
        bit scratch             ; +3
        bit scratch             ; +3
        nop                     ; +2
        ldx idx
        beq ARNoPad
ARPad:  dex                     ; 5 cycles a turn: 2 (dex) + 3 (bne)
        bne ARPad
ARNoPad:
        lda #$00
        sta PF0                 ; SWEPT store: idx 0 -> cy ~43 ... idx 6 -> cy ~71
                                ; (counted, then measured: see the band's result)

        ; ---- line 2: read last line's latch, count the step, advance ----
        sta WSYNC
        bit CXP1FB              ; N = P1/PF from the line just drawn
        bmi ARSet
        lda #0          ; A=0 here is also the beq's condition below - change one, check both
        beq ARPut
ARSet:  lda #1
ARPut:  sta cur
        ldx idx
        sta $A0,x       ; ★診断: 16点の列そのもの
        cmp prev
        beq ARSame
        ldx idx
        cpx #0                  ; the first point has no predecessor
        beq ARSame
        inc stepn
        stx steppos
ARSame: lda cur
        sta prev
        inc idx
        dey
        bne ARLoop

        lda stepn
        sta arSteps
        lda steppos
        sta arPos

; --- the rest of the visible area, dark ---
        lda #$00
        sta PF0
        ldx #163
Dark:   sta WSYNC
        dex
        bne Dark

; --- Overscan ---
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS

        jmp Main

; ---------------------------------------------------------------------------
; Read0 / Read1 — one calibration point: two lines, the second returns 0 or 1 in A.
; ---------------------------------------------------------------------------
Read0:
        sta WSYNC
        sta CXCLR
        sta WSYNC
        bit CXP0FB
        bmi R0Set
        lda #0          ; A=0 here is also the beq's condition below - change one, check both
        beq R0End
R0Set:  lda #1
R0End:  rts

Read1:
        sta WSYNC
        sta CXCLR
        sta WSYNC
        bit CXP1FB
        bmi R1Set
        lda #0          ; A=0 here is also the beq's condition below - change one, check both
        beq R1End
R1Set:  lda #1
R1End:  rts

; ---------------------------------------------------------------------------
; Split1 / Split0 — one line where the two PF0 copies DISAGREE, read on the next.
; The second write lands in the blind gap between the copies (cy ~50).
; ---------------------------------------------------------------------------
Split1:                         ; left lit, right blank
        sta WSYNC
        sta CXCLR
        lda #$F0
        sta PF0                 ; lit, before the left copy
        jsr GapPad              ; ~cy 50
        lda #$00
        sta PF0                 ; blanked before the right copy is drawn
        sta WSYNC
        bit CXP1FB
        bmi S1Set
        lda #0          ; A=0 here is also the beq's condition below - change one, check both
        beq S1End
S1Set:  lda #1
S1End:  rts

Split0:                         ; left blank, right lit
        sta WSYNC
        sta CXCLR
        lda #$00
        sta PF0
        jsr GapPad
        lda #$F0
        sta PF0
        sta WSYNC
        bit CXP1FB
        bmi S0Set
        lda #0          ; A=0 here is also the beq's condition below - change one, check both
        beq S0End
S0Set:  lda #1
S0End:  rts

GapPad:                         ; burn to about cycle 50 of the current line
        ldx #6
GPl:    dex
        bne GPl
        rts

        org $FFFC
        .word Reset
        .word Reset
