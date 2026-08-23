; pf_wraps — the third witness for the playfield-DEADLINE check, and the one that shows why
; "did every write land in time?" is not a question you can always answer.
;
; Colour clocks fold back every 228, so a write pushed a whole scanline late reappears as a
; SMALL clock in the next line's HBLANK and compares as comfortably early. Measured 2026-08-23
; by adding nops at the head of a play region in a real work:
;
;     +10 nops (96 cycles > 76)   ->  6 of 23 writes LATE
;     +26 nops (128 > 76)         ->  3 of 23 LATE
;     +40 nops (156 > 76)         ->  ok, all 23 land in time
;
; Breaking the kernel HARDER made the verdict greener, and the worst one was green. This file is
; that shape, minimised: pf_ontime with forty nops at the top of the line, so every playfield
; store lands on the FOLLOWING scanline.
;
; The check must not call these writes on time, and must not call them late either -- neither is
; true. It reports them as NOT JUDGED, because MinAbs/MaxAbs count from the region's own WSYNC
; and anything at 228 or beyond is being compared against the deadlines of a line it is not on.
;
; The discriminator is deliberately NOT "does the region fit its budget". That is a proxy, and
; cmd/cyclebound uses it: it refuses to run this check at all on an uncertified kernel, which
; also silences it for a kernel that overruns by one cycle and whose writes are all still in
; their own line. The measured fact is available per write and costs nothing.
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
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
        nop                     ; x40: push every store past this line
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
