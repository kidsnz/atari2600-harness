; litmus_mirror — hardware verification of the address mirrors (V2-12, woodgrain Memory_Map)
; RAM: $0180 is a mirror of zero-page RAM $80 (= why the stack works). TIA: $0049 is a mirror of COLUBK ($09).
; RAM map:
;  $90 = write $5A to $0180 → read $0080 (expect $5A: the mirror holds)
;  $91 = write $A5 to $0080 → read $0180 (expect $A5: it holds in the other direction too)
; TIA: set the visible-area background to $84 (blue) **through the mirror $0049** → if read_row's background is blue the TIA mirror holds.
; Hardware-verified (Gopher2600, v0.49.0): $90=$5A (write $0180 → read $0080) / $91=$A5 (other direction) = RAM $0180 is a mirror of $0080
; (why the stack works). The background becomes $84 blue through the TIA mirror $0049 = read_row(100)=$84. Regression-locked = scenarios/mirror.json.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK_M = $49          ; TIA mirror of COLUBK ($09)
RAM_M    = $0180        ; RAM mirror of $80 (stack page)

        org $F000
Start:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        sta $2C             ; CXCLR

        ; --- RAM mirror ---
        lda #$5A
        sta RAM_M           ; write to $0180
        lda $80             ; read $0080
        sta $90             ; expect $5A
        lda #$A5
        sta $80             ; write to $0080
        lda RAM_M           ; read $0180
        sta $91             ; expect $A5
        ; (kept at separate addresses so $90/$91 are not overwritten by $A5)

NextFrame:
        lda #2
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
        lda #0
        sta VBLANK
        ; --- visible: background to $84 (blue) through the TIA mirror $0049 ---
        lda #$84
        sta COLUBK_M        ; mirror write
        ldy #192
Vis:    sta WSYNC
        dey
        bne Vis
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
