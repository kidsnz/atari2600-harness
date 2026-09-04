; litmus_flicker_attrib.asm — do the collision latches attribute to the object drawn
; THIS frame, when two objects share one slot on alternate frames?
;
; fundamentals-audit called this "a verifiable pattern once we do flicker". We have
; flicker (roms/techniques/flicker_multiplex.asm, technique #10) and it touches no
; collision register at all, so the pattern was never built. This ROM builds it.
;
; The interesting claim is not "a collision latches" — litmus_cxclr has that. It is
; that a flickered slot's latch belongs to whichever object was drawn in the frame you
; read it, which is only true if CXCLR runs every frame. Showing that needs a control
; where CXCLR does NOT run, and the control's expected result depends on the latches
; surviving a frame boundary — which nothing here had measured. litmus_cxclr takes its
; three snapshots inside ONE frame (its own header says so) and strobes CXCLR every
; frame, so a latch never gets the chance to cross a boundary there.
;
; So group 1 measures that first, in this ROM, and the control then rests on our own
; measurement instead of an assumption.
;
;   group 1 ($90-$93, 4 frames)  latches survive a frame boundary; only CXCLR clears
;   group 2 ($A0-$A7 + $B0-$B7)  with CXCLR every frame, the latch follows the frame
;   group 3 ($C0-$C7)            without CXCLR, attribution is lost (all frames read set)
;   group 4 ($D0-$D7)            phase inverted, so "even frame" is not the cause
;
; Two things are deliberate:
;
; * Collision is switched by BLANKING THE PLAYFIELD, not by moving P0. litmus_cxclr's
;   header records losing a day to a positioning bug that made all three snapshots read
;   "no collision"; its lesson is "the fixture is about the latches, not about
;   positioning, take the positioning out of the answer". P0 is placed once and never
;   moves. (Design said move P0; this is the one place the ROM departs from it.)
;
; * Every stored byte is NORMALISED to 0 or 1 before it lands in RAM. Raw CXP0FB must
;   not be stored: only D7/D6 are driven, and Gopher2600 fills the rest from the last
;   value the CPU put on the bus (memory.go "data |= mem.LastCPUData & ^mem.DataBusDriven"),
;   which is why scenarios/litmus_cxclr.json pins 130 and 2 rather than 128 and 0 — and
;   why a harmless reorder of that ROM's instructions would fail it without the TIA doing
;   anything different. Normalising means the scenario pins the TIA and nothing else.
;
; Self-contained (no vcs.h). NTSC frame = 262 lines, MEASURED, not derived: the four
; blocks below strobe WSYNC 3 + 36 + 192 + 30 = 261 times and the frame comes out 262,
; because the line the VSYNC block starts on is not one any of them closes. Writing
; 3/37/192/30 from the arithmetic gave 263. The number here is what StepFrame returned. 28 frames of measurement.
; Design by the mailing-list distillation (helper-3), 2026-09-03.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUPF  = $08
COLUBK  = $09
CTRLPF  = $0A
PF0     = $0D
PF1     = $0E
PF2     = $0F
RESP0   = $10
GRP0    = $1B
CXCLR   = $2C
CXP0FB  = $02          ; read: D7 = P0/PF, D6 = P0/BL

frame   = $80          ; 0..27, then the ROM idles
scratch = $81

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
        sta CXCLR       ; start from a known-clear latch set
        sta frame

        lda #$0E
        sta COLUP0
        sta COLUPF
        lda #$00
        sta COLUBK
        sta CTRLPF      ; repeat, not reflect
        sta NUSIZ0
        lda #$FF
        sta GRP0        ; P0 solid 8px

Main:
; --- VSYNC: 3 lines ---
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 36 lines. Decide this frame's CXCLR and playfield, then place P0. ---
        lda #2
        sta VBLANK
        sta WSYNC       ; VBLANK line 1 — closes the line the VSYNC block ended on, so
                        ; FrameSetup below starts at a line boundary.

        jsr FrameSetup
        sta WSYNC       ; VBLANK line 2. This WSYNC is what makes the frame a constant
                        ; length: FrameSetup takes a different number of cycles in each
                        ; group, and without a WSYNC to absorb that the frame measured
                        ; 260 lines in one frame and 261 in the next. A subroutine that
                        ; runs outside a WSYNC puts its own branch structure into the
                        ; scanline count.

        ldx #34
VB:     sta WSYNC       ; VBLANK lines 2..35
        dex
        bne VB

        lda #40         ; placed once, in every frame, at the same value
        sec
Div15:  sbc #15
        bcs Div15
        sta RESP0
        lda #0
        sta VBLANK
        sta WSYNC       ; VBLANK line 36

; --- Visible: 192 lines. P0 sits where the playfield is lit, or is not. ---
        ldx #192
Vis:    sta WSYNC
        dex
        bne Vis

; --- Overscan: 30 lines. Read the latch on the first, store 0 or 1. ---
        lda #2
        sta VBLANK
        sta WSYNC       ; overscan line 1 — Record must start at a line boundary too
        jsr Record
        sta WSYNC       ; overscan line 2
        jsr RecordCause
        sta WSYNC       ; overscan line 3

        ldx #27
OS:     sta WSYNC       ; overscan lines 4..30
        dex
        bne OS

        inc frame
        jmp Main

; ---------------------------------------------------------------------------
; FrameSetup — strobe CXCLR if this frame calls for it, then light or blank PF.
; ---------------------------------------------------------------------------
FrameSetup:
        lda frame
        cmp #4
        bcc G1Set               ; frames 0-3   : group 1
        cmp #12
        bcc G2Set               ; frames 4-11  : group 2
        cmp #20
        bcc G3Set               ; frames 12-19 : group 3
        cmp #28
        bcc G4Set               ; frames 20-27 : group 4
        rts                     ; past the end: leave everything alone

; group 1: F0 clear+lit, F1 lit (no clear), F2 blank (no clear), F3 clear+blank
G1Set:
        cmp #1
        beq NoClearLit
        cmp #2
        beq NoClearBlank
        cmp #3
        beq ClearBlank
        ; frame 0
        sta CXCLR
        jmp LightPF
NoClearLit:
        jmp LightPF
NoClearBlank:
        jmp BlankPF
ClearBlank:
        sta CXCLR
        jmp BlankPF

; group 2: CXCLR every frame; lit on even offsets, blank on odd
G2Set:
        sta CXCLR
        sec
        sbc #4
        jmp EvenLit

; group 3: CXCLR on the first frame only; same alternation as group 2
G3Set:
        sec
        sbc #12
        cmp #0
        bne G3NoClear
        sta CXCLR
G3NoClear:
        jmp EvenLit

; group 4: CXCLR every frame; the alternation is INVERTED
G4Set:
        sta CXCLR
        sec
        sbc #20
        lsr                     ; C = bit0 of the offset
        bcs LightPF             ; odd offset -> lit  (inverted vs groups 2/3)
        jmp BlankPF

; A = an offset; light the playfield on even offsets, blank it on odd.
EvenLit:
        lsr                     ; C = bit0
        bcs BlankPF
LightPF:
        lda #$F0
        sta PF0
        lda #$FF
        sta PF1
        sta PF2
        rts
BlankPF:
        lda #0
        sta PF0
        sta PF1
        sta PF2
        rts

; ---------------------------------------------------------------------------
; Record — normalise CXP0FB's D7 to 0/1 and store it where this frame belongs.
; Group 2 also records which object was drawn, so the two can be compared.
; ---------------------------------------------------------------------------
Record:
        lda frame
        cmp #28
        bcs RecDone             ; past the end: stop writing

        ; X = the RAM cell for this frame
        cmp #4
        bcs R2
        clc
        adc #$90                ; group 1 -> $90..$93
        tax
        jmp RecStore
R2:     cmp #12
        bcs R3
        sec
        sbc #4
        clc
        adc #$A0                ; group 2 -> $A0..$A7
        tax
        jmp RecStore
R3:     cmp #20
        bcs R4
        sec
        sbc #12
        clc
        adc #$C0                ; group 3 -> $C0..$C7
        tax
        jmp RecStore
R4:     sec
        sbc #20
        clc
        adc #$D0                ; group 4 -> $D0..$D7
        tax

RecStore:
        bit CXP0FB              ; N = D7 (P0/PF); V = D6 (P0/BL), unused here
        bmi RecSet
        lda #0          ; A=0 here is also the beq's condition below - change one, check both
        beq RecPut
RecSet: lda #1
RecPut: sta $00,x
RecDone:
        rts

; ---------------------------------------------------------------------------
; RecordCause — group 2 only: store whether the playfield was lit this frame, so
; the scenario can assert the latch and its cause agree cell by cell.
;
; This is a SEPARATE subroutine, called from its own line, because folding it into
; Record made group 2's frames one scanline longer than every other group's: the
; longer path overflowed the line the caller had opened. A subroutine whose length
; depends on which case it is in cannot share a line with another one.
; ---------------------------------------------------------------------------
RecordCause:
        lda frame
        cmp #4
        bcc RCDone
        cmp #12
        bcs RCDone
        sec
        sbc #4
        tax                     ; X = 0..7
        txa
        lsr
        bcs RCBlank             ; odd offset -> blank -> 0
        lda #1
        bne RCPut               ; always taken: A=1 here, see the lda above
RCBlank:
        lda #0
RCPut:  sta $B0,x
RCDone:
        rts

        org $FFFC
        .word Reset
        .word Reset
