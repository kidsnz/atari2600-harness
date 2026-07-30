; litmus_bank_shared_addr — F8 (8K, 2 banks): TWO BANKS EXECUTE DIFFERENT CODE WITH
; DIFFERENT CYCLE COSTS AT THE SAME ADDRESS, inside ONE WSYNC-to-WSYNC region.
;
; Why it exists. Every other bank ROM in this corpus is measured to never execute the
; same address in two banks (banksound_test.go prints that per ROM), so none of them
; can tell a prover keyed on (bank, address) from one keyed on the bare address: a flat
; map keeps whichever bank was inserted last, and on these ROMs the two never collide
; where it matters. Without this ROM, every site-keyed map in internal/cyclebound is
; untested against the failure it exists to prevent.
;
; The shape. The overscan region calls a chain that runs $FF10 and $FF12 in bank 0, then
; switches and runs DIFFERENT instructions at those SAME addresses in bank 1:
;
;   addr    bank 0            bank 1
;   $FF10   lda #$A0   2cy    inc $8C    5cy
;   $FF12   sta $8D    3cy    lda #$B1   2cy
;
; A flat-keyed prover holds only one of each pair, so its worst case is wrong by
; (5-2) + (2-3) = +2 or -2 depending on which bank won the insertion race — and a
; prover that is wrong DOWNWARD is the failure this package forbids. The test asserts
; the proven worst equals the emulator's measured worst, which pins both.
;
; Runtime evidence that both banks really run: $8D ends every frame at $B1 (bank 1
; wrote it last) and $8C increments once per frame (only bank 1 increments it).
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02

; ================= bank 0 =================
        ORG  $0000
        RORG $F000
Start:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

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
        ldy #192
Vis:    sta WSYNC
        dey
        bne Vis
        lda #2
        sta VBLANK
        jsr $FF00           ; the shared-address chain (44cy: 18 in bank0, 20 in bank1, 6 rts)
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

        ; --- shared-address zone, bank 0 side ---
        ORG  $0F00
        RORG $FF00
        jmp $FF10           ; $FF00-02
        ds 13, $EA          ; $FF03-0F
        lda #$A0            ; $FF10-11  2cy  <-- SHARED ADDRESS
        sta $8D             ; $FF12-13  3cy  <-- SHARED ADDRESS
        lda $FFF9           ; $FF14-16  4cy  hotspot -> next fetch is bank1 $FF17
        ds 8, $EA           ; $FF17-1E  bank 1 executes these addresses, not these bytes
        rts                 ; $FF1F     6cy  bank 1 returns here (bank 0)

        ; --- reset stub, identical in both banks and at the same address ---
        ORG  $0FE0
        RORG $FFE0
        lda $FFF8           ; whichever bank powers on, select bank 0
        jmp $F000

        ORG  $0FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 1 =================
        ORG  $1000
        RORG $F000
        ds 3, 0             ; bank 1's $F000 is unused

        ; --- shared-address zone, bank 1 side ---
        ORG  $1F00
        RORG $FF00
        ds 16, $EA          ; $FF00-0F  bank 0 executes these addresses, not these bytes
        inc $8C             ; $FF10-11  5cy  <-- SHARED ADDRESS (bank 0 has lda #, 2cy)
        lda #$B1            ; $FF12-13  2cy  <-- SHARED ADDRESS (bank 0 has sta zp, 3cy)
        jmp $FF1A           ; $FF14-16  3cy  hop over bank 1's own entry stub
        jmp $FF10           ; $FF17-19  3cy  ENTRY: the landing of bank 0's hotspot read
        sta $8D             ; $FF1A-1B  3cy  $8D = $B1 => bank 1 ran
        lda $FFF8           ; $FF1C-1E  4cy  hotspot -> next fetch is bank0 $FF1F (rts)

        ; --- reset stub, identical to bank 0's ---
        ORG  $1FE0
        RORG $FFE0
        lda $FFF8
        jmp $F000

        ORG  $1FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0
