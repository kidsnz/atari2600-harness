; litmus_cxclr.asm — the collision latches: sticky, cleared by CXCLR, and NOT by HMCLR.
;
; CLAUDE.md states all three under "constants you must never get wrong": "two latches
; in each D7/D6, sticky" and "CXCLR = clear all collisions; HMCLR = clear the motion
; registers (a DIFFERENT thing)". The bit assignment is locked by a pure-function test.
; The other three claims were not: stickiness appears only in a comment, and nothing
; anywhere checks that HMCLR leaves collisions alone — which is the one a reader can
; actually get wrong, since the names differ by two letters and both "clear something".
;
; The kernel drives P0 across a lit playfield so CXP0FB's D7 (P0/PF) latches, then in
; overscan snapshots the register into RAM at three points:
;
;   $80 — after the collision, before anything is cleared   → D7 set
;   $81 — after writing HMCLR                               → D7 STILL set
;   $82 — after writing CXCLR                               → cleared
;
; RAM is used rather than a live read because these are write-only strobes: poking them
; from outside does not persist (CLAUDE.md's poke quirk), so the ROM has to do it.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

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
HMCLR   = $2B
CXCLR   = $2C
CXP0FB  = $02          ; read: D7 = P0/PF, D6 = P0/BL

snap0   = $80          ; latch state before any clear
snap1   = $81          ; after HMCLR
snap2   = $82          ; after CXCLR

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

        lda #$0E
        sta COLUP0      ; P0 white
        sta COLUPF      ; playfield white
        lda #$00
        sta COLUBK
        sta CTRLPF      ; repeat, not reflect
        sta NUSIZ0
        lda #$FF
        sta GRP0        ; P0 solid 8px
        ; The WHOLE left half lit (and repeated right), so the collision does not
        ; depend on where P0 lands. A first version lit PF1 only and positioned P0
        ; with a div-15 loop that was never given a target value — A still held 2
        ; from the VBLANK write — so P0 sat near the left edge, missed the band, and
        ; all three snapshots read "no collision". The fixture is about the latches,
        ; not about positioning; take the positioning out of the answer.
        lda #$F0
        sta PF0
        lda #$FF
        sta PF1
        sta PF2

Main:
; --- VSYNC: 3 lines ---
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 37 lines, last one places P0 over the playfield ---
        lda #2
        sta VBLANK
        ldx #36
VB:     sta WSYNC
        dex
        bne VB

        sta WSYNC
        lda #40         ; anywhere inside the lit half; the playfield is solid there
        sec
Div15:  sbc #15
        bcs Div15
        sta RESP0
        lda #0
        sta VBLANK

; --- Visible: 192 lines. P0 sits on the playfield, so CXP0FB D7 latches. ---
        ldx #192
Vis:    sta WSYNC
        dex
        bne Vis

; --- Overscan: 30 lines, with the three snapshots taken on the first ---
        lda #2
        sta VBLANK

        lda CXP0FB
        sta snap0       ; collided, nothing cleared yet

        sta HMCLR       ; the motion registers — NOT the collision latches
        lda CXP0FB
        sta snap1       ; must be unchanged

        sta CXCLR       ; the collision latches
        lda CXP0FB
        sta snap2       ; must be clear

        ldx #30
OS:     sta WSYNC
        dex
        bne OS

        jmp Main

        org $FFFC
        .word Reset
        .word Reset
