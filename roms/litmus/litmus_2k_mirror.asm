; litmus_2k_mirror — the 2K mirror creates TWO separate traps, not one, and they are different bytes.
;
; `capability-gap-audit.md` CMB-6 says Combat "squats live game data on the CPU IRQ/BRK vector slot"
; and that "a 2K cart mirrors $F000-$F7FF into $F800-$FFFF, so $F7FE/$F7FF *are* the $FFFE/$FFFF
; vector ... This booby-traps a 2K->4K port."
;
; The archive says something that sounds like the same thing pointing the other way:
;
;	1997-03, Chris Pepin: "That's one of the problems I was having with games not working.
;	                       When I doubled them up to 4k, they worked fine."
;	2003-10 (via helper-1's note): a 2K game's `BRK` reads its vector from $FFFE/$FFFF, which on a
;	                       Supercharger is $1FF8/$1FF9 — the control hotspots.
;
; Same mirroring, opposite conclusions: "2K breaks, 4K fixes it" against "2K->4K is the trap". ★So
; is it one mechanism or two? **It is two, and the address decides.** Under the same 2K mirror the
; two hazards sit on *different byte pairs*:
;
;	file offset $7FE/$7FF  ->  $F7FE/$F7FF  ==  $FFFE/$FFFF   the IRQ/BRK vector      (CMB-6)
;	file offset $7F8/$7F9  ->  $F7F8/$F7F9  ==  $FFF8/$FFF9   the Supercharger hotspot
;
; Six bytes apart. A 2K image can trip either, both, or neither, and "double it to 4K" only helps
; the second: at 4K the top half is no longer a mirror of the bottom, so the bytes that were landing
; on $1FF8/$1FF9 stop doing so — while the vector slot at $FFFE/$FFFF is still whatever the author
; put there. **That is why the two reports do not contradict each other.**
;
; This ROM is 2K and measures the mirror itself, which is the part both claims rest on:
;
;	$80  the byte at $F7F8   $82  the byte at $FFF8   (must be equal — the SC-hotspot pair)
;	$81  the byte at $F7FE   $83  the byte at $FFFE   (must be equal — the vector pair)
;	$84  1 if the two pairs hold DIFFERENT markers, i.e. they are distinct locations
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

        org $F800           ; ★2K: this image is mirrored into $F000-$F7FF as well
Start:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

        lda $F7F8
        sta $80
        lda $F7FE
        sta $81
        lda $FFF8
        sta $82
        lda $FFFE
        sta $83
        lda $80
        cmp $81
        beq Same            ; ★the two markers must differ, or the test proves nothing
        lda #1
        sta $84
Same:

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

        ; --- the two hazard sites, six bytes apart, given distinguishable markers ---------
        org $FFF8
        .byte $5C, $5C      ; the Supercharger control hotspots on a 2K image
        .word Start         ; $FFFA NMI
        .word Start         ; $FFFC RESET
        .byte $A3, $A3      ; $FFFE IRQ/BRK — CMB-6's "live game data on the vector slot"
