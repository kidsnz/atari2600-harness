; tiaprobe.asm — calibration ROM for the Stella TIA write-register oracle (G4).
;
; Purpose: establish, from the ROM SOURCE alone, what Stella's `tia` debugger
; command means by each field — is `HM=$7` the raw HMxx nibble or a normalised
; motion? is a missile `size=#1` the 2-bit NUSIZ field or a pixel width? does
; `GR=` print the NEW graphics register or the VDEL-selected one? Reading those
; conventions off our own emulator would make the cross-check circular, so this
; ROM writes a distinct, asymmetric constant to every comparable write-only
; register and then never touches TIA again. Whatever Stella prints at any frame
; boundary is therefore a statement about the constants written HERE.
;
; Source of the constants: this file. Nothing is derived from Gopher2600.

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
REFP0   = $0B
REFP1   = $0C
PF0     = $0D
PF1     = $0E
PF2     = $0F
GRP0    = $1B
GRP1    = $1C
ENAM0   = $1D
ENAM1   = $1E
ENABL   = $1F
HMP0    = $20
HMP1    = $21
HMM0    = $22
HMM1    = $23
HMBL    = $24
VDELP0  = $25
VDELP1  = $26
VDELBL  = $27
RESMP0  = $28
RESMP1  = $29

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
ClearMem:
        sta $00,x
        dex
        bne ClearMem

; --- colours: four distinct, none of them 0 ---
        lda #$32
        sta COLUP0
        lda #$54
        sta COLUP1
        lda #$76
        sta COLUPF
        lda #$98
        sta COLUBK

; --- NUSIZ: player copy mode and missile size differ per object ---
        lda #$16                ; P0: mode 6 (3 copies - med), M0 size field 1
        sta NUSIZ0
        lda #$25                ; P1: mode 5 (2x sized player), M1 size field 2
        sta NUSIZ1

; --- CTRLPF: reflect+score+priority all set, ball size field = 2 ---
        lda #$27
        sta CTRLPF

; --- reflection: P0 reflected, P1 not ---
        lda #$08
        sta REFP0
        lda #$00
        sta REFP1

; --- playfield bytes: three distinct values ---
        lda #$B0
        sta PF0
        lda #$C3
        sta PF1
        lda #$5A
        sta PF2

; --- vertical delay: P0 and BL delayed, P1 not ---
        lda #$01
        sta VDELP0
        lda #$00
        sta VDELP1
        lda #$01
        sta VDELBL

; --- graphics: leave GRP0's old and new copies DIFFERENT.
;     A write to GRP0 copies GRP1(new) into GRP1(old); a write to GRP1 copies
;     GRP0(new) into GRP0(old). After this sequence:
;       GRP0 new=$A5 old=$22 , GRP1 new=$3C old=$3C
;     so what Stella prints for P0 says which copy it reports.
        lda #$22
        sta GRP0
        lda #$3C
        sta GRP1
        lda #$A5
        sta GRP0

; --- enables: M0 and BL on, M1 off ---
        lda #$02
        sta ENAM0
        lda #$00
        sta ENAM1
        lda #$02
        sta ENABL

; --- missile-to-player lock: M0 on, M1 off ---
        lda #$02
        sta RESMP0
        lda #$00
        sta RESMP1

; --- horizontal motion registers: five distinct nibbles, 7/F/8/1/D.
;     HMOVE is never strobed, so nothing moves; the registers just hold.
        lda #$70
        sta HMP0
        lda #$F0
        sta HMP1
        lda #$80
        sta HMM0
        lda #$10
        sta HMM1
        lda #$D0
        sta HMBL

; --- from here on the only TIA writes are VSYNC/VBLANK/WSYNC ---
MainLoop:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

        ldx #37
VBlankLoop:
        sta WSYNC
        dex
        bne VBlankLoop
        lda #0
        sta VBLANK

        ldx #192
VisibleLoop:
        sta WSYNC
        dex
        bne VisibleLoop

        lda #2
        sta VBLANK
        ldx #30
OverscanLoop:
        sta WSYNC
        dex
        bne OverscanLoop

        jmp MainLoop

        org $FFFA
        .word Reset
        .word Reset
        .word Reset
