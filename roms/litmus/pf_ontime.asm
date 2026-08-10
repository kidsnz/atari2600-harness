; pf_ontime — a witness pair for the playfield-DEADLINE check.
;
; Both draw the same 40-column asymmetric playfield and both fit inside 76 cycles. The
; difference is three cycles of arithmetic at the TOP of the line in pf_late, before the
; first store, which pushes every write past the clock the beam reaches the pixels it
; governs. That is not a cost problem and cyclebound CERTIFIES both: a region can fit and
; still be late, and the picture comes out shifted right with the previous line's right
; edge wrapping in at the left.
;
; The pair exists because that shipped. Measured on the real kernel it was 10 colour
; clocks, two and a half columns.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUPF  = $08
COLUBK  = $09
CTRLPF  = $0A
PF0     = $0D
PF1     = $0E
PF2     = $0F
fudge   = $80
        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        sta CTRLPF              ; repeat, which is what an asymmetric playfield needs
        lda #$0E
        sta COLUPF
Frame:  lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        ldx #0
        stx VBLANK
        ldx #$40
        txa
        tay
        jmp Line
        align 64
Line:   sta WSYNC
        lda TabL-$40,y          ; the same head the real kernel has: a per-line colour
        clc                     ; plus the drum's lift, twelve cycles before PF0 can go
        adc fudge
        sta COLUPF
        lda Tab0-$40,y
        sta PF0
        lda Tab1-$40,y
        sta PF1
        lda Tab2-$40,y
        sta PF2
        lda Tab3-$40,y
        sta PF0
        lda Tab4-$40,y
        sta PF1
        lda Tab5-$40,y
        sta PF2
        inx
        beq Done
        txa                     ; the same arithmetic, in the tail where there is room
        eor fudge
        tay
        jmp Line
Done:   sta WSYNC
        lda #2
        sta VBLANK
        lda #0
        sta PF0
        sta PF1
        sta PF2
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        jmp Frame
        org $F840
TabL:   ds 192, $0C
        org $F940
Tab0:   ds 192, $50
        org $FA40
Tab1:   ds 192, $A5
        org $FB40
Tab2:   ds 192, $5A
        org $FC40
Tab3:   ds 192, $A0
        org $FD40
Tab4:   ds 192, $3C
        org $FE40
Tab5:   ds 192, $C3
        org $FFFC
        .word Start
        .word Start
