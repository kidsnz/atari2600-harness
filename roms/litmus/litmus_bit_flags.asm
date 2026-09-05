; litmus_bit_flags — BIT sets THREE flags from two different places, and the docs here name two.
;
; `docs/resources.md` says: *"8 read-only registers, each with two latches in D7/D6, sticky. Test
; with `BIT CXxx` -> `BMI`(D7)/`BVS`(D6)."* True, and it stops one flag short.
;
; Bill Heineman, stella-list 200207/msg00281:
;
;	Moves Bit #7 from the memory location and places it in the N flag ...
;	Moves Bit #6 from the memory location and places it in the V flag ...
;	[A AND memory] is placed in the Z flag ... the accumulator is NOT affected in anyway.
;	This way, you can do a quick bit test without [damaging the accumulator]
;
; So one `BIT` answers three questions at once, from two different sources:
;
;	N <- memory bit 7        (the memory's own bit, whatever A holds)
;	V <- memory bit 6        (likewise)
;	Z <- (A AND memory) == 0 (a MASK TEST against the accumulator)
;
; ★The third is the useful one this repository does not use: it tests an arbitrary mask without
; touching A, where `AND` would destroy it. And it lands exactly on the register measured in
; `litmus_timint_pa7`: TIMINT's D7 is the timer flag and D6 is the PA7 flag, so
; `BIT TIMINT / BMI expired / BVS pa7` reads both in one instruction.
;
; ★★And a limit worth stating with it, from the same thread: **BIT has no immediate mode**
; (Chris Wilkson, 199806/msg00118) — the mask must live in memory. Andrew Davie corrected himself
; in that thread after saying otherwise: *"my memory got mixed up with the 65816"*.
;
;	$80  N after `BIT` on a byte with bit7 set, bit6 clear   (want 1)
;	$81  V after the same                                     (want 0)
;	$82  N after a byte with bit7 clear, bit6 set             (want 0)
;	$83  V after the same                                     (want 1)
;	$84  Z after `BIT` with A and the byte sharing no bits    (want 1)
;	$85  Z after `BIT` with A and the byte sharing one bit    (want 0)
;	$86  A afterwards, to show BIT did not touch it           (want $0F)
;	$87  N from a byte whose bit7 is set while A is ZERO      (want 1 — N comes from memory,
;	                                                           not from the AND result)
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

hi      = $90       ; $80: bit7 set, bit6 clear
lo      = $91       ; $40: bit7 clear, bit6 set
msk     = $92       ; $0F

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

        lda #$80
        sta hi
        lda #$40
        sta lo
        lda #$0F
        sta msk

        ; --- N and V come from the MEMORY's bits 7 and 6 ---------------------------------
        lda #$FF            ; A is all ones, so the AND can never make Z
        bit hi
        php
        pla
        and #$80
        beq N1clr
        lda #1
N1clr:  sta $80             ; ★sta of 1 only on the taken path; 0 stays from Clr
        lda #$FF
        bit hi
        php
        pla
        and #$40
        beq V1clr
        lda #1
V1clr:  sta $81

        lda #$FF
        bit lo
        php
        pla
        and #$80
        beq N2clr
        lda #1
N2clr:  sta $82
        lda #$FF
        bit lo
        php
        pla
        and #$40
        beq V2clr
        lda #1
V2clr:  sta $83

        ; --- Z is the MASK TEST: (A AND memory) == 0 --------------------------------------
        lda #$F0            ; shares no bits with $0F
        bit msk
        beq Z1set
        jmp Z1done
Z1set:  lda #1
        sta $84
Z1done:

        lda #$0F            ; shares all four low bits with $0F
        bit msk
        bne Z2clr
        jmp Z2done          ; (Z set would be wrong; leave $85 at 0 ... )
Z2clr:  lda #1
        sta $85             ; ★$85 = 1 means Z was CLEAR, which is the correct answer
Z2done:

        ; --- and BIT left the accumulator alone -------------------------------------------
        lda #$0F
        bit hi              ; hi = $80: A AND $80 = 0, so Z sets — and A must be untouched
        sta $86

        ; --- N comes from memory even when the AND result is zero -------------------------
        lda #$00            ; A is zero, so (A AND memory) is zero and Z will set
        bit hi              ; hi has bit7 set: N must still be 1
        php
        pla
        and #$80
        beq N3clr
        lda #1
N3clr:  sta $87

Frame:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        ldx #192
Vis:    sta WSYNC
        dex
        bne Vis
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp Frame

        org $FFFC
        .word Start
        .word Start
