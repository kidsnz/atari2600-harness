; stack_overlap_trap -- PLANTED: one RAM byte is both a variable and part of the stack.
;
; Count lives at $FD. Each frame the main loop counts in it and shows it as the background
; colour; then it calls two subroutines deep, and the second return address lands on $FD
; and $FC. The program does not crash and the picture is stable -- the variable is simply
; no longer what was counted -- which is how this fault passed every check in a real work
; for two steps (roms/261011_edo, steps 15 and 16: a scene builder eight bytes deep under
; variables that were set again for every zone). WatchStackOverlap must FLAG $FD.
;
; The clear at reset pushes zeros through all of RAM on purpose; that is not the fault, and
; the watch starts after the reset code for that reason.

        processor 6502
VSYNC   = $00
WSYNC   = $02
COLUBK  = $09
Count   = $FD

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
Outer:  jsr Inner               ; pushes $FD, $FC: over the variable
        rts
Inner:  rts

        org $FFFC
        .word Reset
        .word Reset
