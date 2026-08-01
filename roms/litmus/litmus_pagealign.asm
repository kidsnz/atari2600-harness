; litmus_pagealign — a page-ALIGNED table cannot be crossed, whatever the index is.
;
; Sibling of litmus_pagecross, and a different branch of the same costing. That one
; pins the case where the index is a known CONSTANT; this one pins the case where the
; index is UNKNOWN — which is the case that actually costs something, because an
; unproven index is exactly when a table gets aligned in the first place.
;
; THE RULE. A 6502 index register holds 0..255, so `$NN00 + idx` is at most `$NNFF`
; and never leaves the base's page. Alignment therefore settles the question without
; any index analysis at all. The prover used to reach its conservative +1 first and
; charge a cycle the hardware cannot spend.
;
; THE FIXTURE. Two WSYNC regions run the same four `lda Tbl,y` reads, with y loaded
; from RAM so its range is not provable. The tables differ only in where they start:
;
;   AlnTbl   at $F100, aligned  -> no index can cross; proof is TIGHT
;   SplitTbl at $F0F8, unaligned -> an unknown index COULD cross; proof is CONSERVATIVE
;
; With no controller attached SWCHA reads $FF, so y is 255 at run time. AlnTbl+255 is
; $F1FF, still page $F1 — the machine does not cross, and the proof must not say it
; might. SplitTbl+255 is $F1F7, a page beyond its base, so there the machine really
; does cross and the conservative charge is not merely safe but correct. The split
; region is therefore NOT the place to measure looseness; the test checks that the
; costing function still charges an unaligned base directly instead.
;
; WHY THIS ROM EXISTS AT ALL. Measured 2026-08-01 across the 135 ROMs in roms/: the
; "aligned base, unknown index" case fires ZERO times. That is a fact about the
; corpus, not about the case — 24 of the 31 technique kernels draw no playfield, so
; there is no table-driven picture kernel among them, and a picture kernel is what
; aligns tables. The first one written (an asymmetric-playfield sunset in the
; practice tree) produced eight wasted charges on its first run, and its proven worst
; read 74 against a machine that takes 66: two cycles of headroom reported where
; there were ten. That kernel lives in a different repository, so without this ROM
; the branch would ship with no witness here.
;
; Do NOT subtract the two regions' absolute numbers from each other. They hold the
; same instructions but sit at different addresses, so branch page-crossings alone
; move them by a few cycles — the same warning litmus_pagecross carries. What is
; meaningful is each region against the machine.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
COLUPF  = $08
SWCHA   = $0280         ; the joystick port: a value from OUTSIDE the machine

idx     = $80           ; the index lives in RAM, fed from SWCHA

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

        lda SWCHA
        sta idx         ; an index the analysis CANNOT pin
                        ;
                        ; The first version wrote a constant here, and the premise
                        ; check in the test caught it: the abstract interpreter
                        ; tracks RAM, so `lda #4 / sta idx / ldy idx` proves y == 4
                        ; and all four reads took the already-free "index known"
                        ; path. The fixture looked right and exercised nothing. A
                        ; port read is unknowable by construction.

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
; Region A: four reads from the ALIGNED table.
AlnRow: sta WSYNC
        ldy idx
        lda AlnTbl,y
        sta COLUBK
        lda AlnTbl,y
        sta COLUPF
        lda AlnTbl,y
        sta COLUBK
        lda AlnTbl,y
        sta COLUPF

; Region B: the same four reads from the UNALIGNED table.
SplRow: sta WSYNC
        ldy idx
        lda SplitTbl,y
        sta COLUBK
        lda SplitTbl,y
        sta COLUPF
        lda SplitTbl,y
        sta COLUBK
        lda SplitTbl,y
        sta COLUPF

        lda #0
        sta COLUBK
        sta COLUPF
        ldx #190
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

; SplitTbl deliberately starts 8 bytes below the page boundary, so an index of 248
; or more would reach $F100 and beyond. Nothing here uses such an index; the point
; is that the PROVER cannot know that, and must charge for it.
        org $F0F8
SplitTbl:
        .byte $00,$02,$04,$06,$08,$0A,$0C,$0E

; AlnTbl starts exactly on the page boundary, so $F100+255 = $F1FF is still page $F1.
        org $F100
AlnTbl:
        .byte $40,$42,$44,$46,$48,$4A,$4C,$4E

        org $FFFC
        .word Reset
        .word Reset
