; stack_overlap_clean -- the clean twin of stack_overlap_trap: the same program, the variable out of the way.
;
; Count lives at $80; the two-deep call reaches $FC-$FF and nothing else uses those bytes.
; WatchStackOverlap must say nothing. (The clear at reset still pushes through every byte.)
;
; The clear at reset pushes zeros through all of RAM on purpose; that is not the fault, and
; the watch starts after the reset code for that reason.

        processor 6502
VSYNC   = $00
WSYNC   = $02
COLUBK  = $09
Count   = $80

        org $F000
Reset:
        sei
        cld
        ldx #0
        txa
Clear:  dex
        txs
        pha
        bne Clear               ; SP ends at $FF
Main:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        inc Count               ; the variable, through zero page
        lda Count
        sta COLUBK
        jsr Outer               ; pushes $FF, $FE
        ldx #100
L:      sta WSYNC
        dex
        bne L
        jmp Main
Outer:  jsr Inner               ; pushes $FD, $FC
        rts
Inner:  rts

        org $FFFC
        .word Reset
        .word Reset
