; tiaprobe2.asm — second calibration ROM for the Stella TIA write-register oracle.
;
; Same idea as tiaprobe.asm but with every asymmetric choice MIRRORED: the flag
; that was set on object 0 is set on object 1 instead, the NUSIZ modes are two
; other ones, the colours differ. Its first job is to be an independent fixture
; for the parser. Its second is to settle one disagreement tiaprobe.asm exposed:
; tiaprobe writes RESMP0=$02 / RESMP1=$00 and Stella 7.0 prints RESET (set) on
; BOTH missile lines. If Stella's M1 line is really reporting RESMP0, then with
; RESMP0=$00 / RESMP1=$02 here it must print reset (clear) on both lines; if
; Stella is reading RESMP1 correctly it must print reset on M0 and RESET on M1.
;
; Source of the constants: this file.

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
        lda #$1a
        sta COLUP0
        lda #$2c
        sta COLUP1
        lda #$4e
        sta COLUPF
        lda #$60
        sta COLUBK

; --- NUSIZ: player copy mode and missile size differ per object ---
        lda #$33                ; P0: mode 3 (3 copies - close), M0 size field 3
        sta NUSIZ0
        lda #$07                ; P1: mode 7 (4x sized player), M1 size field 0
        sta NUSIZ1

; --- CTRLPF: reflect+score+priority all set, ball size field = 2 ---
        lda #$12
        sta CTRLPF

; --- reflection: P0 reflected, P1 not ---
        lda #$00
        sta REFP0
        lda #$08
        sta REFP1

; --- playfield bytes: three distinct values ---
        lda #$70
        sta PF0
        lda #$0f
        sta PF1
        lda #$a5
        sta PF2

; --- vertical delay: P0 and BL delayed, P1 not ---
        lda #$00
        sta VDELP0
        lda #$01
        sta VDELP1
        lda #$00
        sta VDELBL

; --- graphics: leave GRP0's old and new copies DIFFERENT.
;     A write to GRP0 copies GRP1(new) into GRP1(old); a write to GRP1 copies
;     GRP0(new) into GRP0(old). After this sequence:
;       GRP0 new=$A5 old=$22 , GRP1 new=$3C old=$3C
;     so what Stella prints for P0 says which copy it reports.
        lda #$44
        sta GRP1
        lda #$81
        sta GRP0
        lda #$5A
        sta GRP1

; --- enables: M0 and BL on, M1 off ---
        lda #$00
        sta ENAM0
        lda #$02
        sta ENAM1
        lda #$00
        sta ENABL

; --- missile-to-player lock: M0 on, M1 off ---
        lda #$00
        sta RESMP0
        lda #$02
        sta RESMP1

; --- horizontal motion registers: five distinct nibbles, 7/F/8/1/D.
;     HMOVE is never strobed, so nothing moves; the registers just hold.
        lda #$30
        sta HMP0
        lda #$90
        sta HMP1
        lda #$B0
        sta HMM0
        lda #$E0
        sta HMM1
        lda #$00
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
