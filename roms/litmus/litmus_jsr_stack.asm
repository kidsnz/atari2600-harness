; litmus_jsr_stack.asm — SD-3: what a JSR does to SP, and what it writes.
;
; Two facts about the stack that a static analysis has to get right, in one frame.
;
;  1. THE ORDINARY IDIOM. `jsr Save` where Save opens with `pha`. The JSR pushes the
;     return address FIRST, so with the caller's SP at $FF the callee's PHA lands at
;     $01FD — not at $01FF. An interpreter that hands the callee the caller's SP
;     unchanged names an address two above the one the machine writes, and names it
;     with `exact: true`. This is the most common subroutine shape on the 6502, so
;     the error is not an exotic corner: it is the default case.
;
;  2. THE JSR'S OWN WRITE. A JSR stores two bytes of memory, and on the 2600 page 1
;     is the same address space the console decodes. With SP aimed at the TIA the
;     return address IS a pair of register writes ($010B = REFP1, $010A = CTRLPF),
;     and the PHA that follows inside the callee writes $0109 = COLUBK. The
;     background goes green halfway down the frame, so the picture arbitrates: a
;     tool that reports no COLUBK write here is contradicted by the screen.
;
; Green enters through a JSR and leaves through a JMP on purpose. Its return address
; is sitting in write-only TIA registers, so there is nothing for an RTS to read
; back — that is a property of the hardware, not a shortcut taken here.
;
; Self-contained (no include), NTSC 262 lines.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
CTRLPF  = $0A
REFP1   = $0B

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

Main:
; --- VSYNC: 3 lines ---
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 37 lines. Fact 1 runs on the first of them. ---
; SP is re-established here rather than assumed: the previous frame left it wherever
; Green put it, and the point of fact 1 is the OFFSET between the caller's SP and the
; callee's, which is only a statement about a known caller SP.
        sta WSYNC
        ldx #$FF
        txs
        jsr Save
        ldx #36
VBlank:
        sta WSYNC
        dex
        bne VBlank
        lda #0
        sta VBLANK

; --- Visible: 96 lines of black, written by an ordinary `sta` the analysis sees ---
        lda #$00
        sta COLUBK
        ldx #96
Top:
        sta WSYNC
        dex
        bne Top

; --- Fact 2: aim SP at the TIA and call. ---
        sta WSYNC
        ldx #$0B
        txs
        jsr Green
AfterGreen:

; --- Visible: 95 more lines, now green ---
        ldx #95
Bottom:
        sta WSYNC
        dex
        bne Bottom

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
Overscan:
        sta WSYNC
        dex
        bne Overscan

        jmp Main

; Fact 1. Called with SP=$FF, so the JSR leaves SP at $FD and this PHA writes $01FD
; (the page-1 mirror of RAM $FD). The PLA puts SP back at $FD for the RTS.
Save:
        lda #$A5
        pha
        pla
        rts

; Fact 2. Entered with SP=$0B, so the JSR wrote $010B and $010A and left SP at $09:
; this PHA writes $0109 = COLUBK. SP is restored and the two registers the return
; address landed on are cleared before the picture continues.
Green:
        lda #$C4
        pha
        ldx #$FF
        txs
        lda #0
        sta CTRLPF
        sta REFP1
        jmp AfterGreen

; --- vectors ---
        org $FFFC
        .word Reset
        .word Reset
