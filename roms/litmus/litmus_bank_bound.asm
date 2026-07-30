; litmus_bank_bound — F8 (8K, 2 banks): a counted loop in bank 1 whose COUNTER IS
; INITIALISED IN BANK 0. The cross-bank sibling of litmus_bound_proxy.asm.
;
; Why it exists. SD-9 removed an address-order proxy from determineBound ("the immediate
; LDX/LDY at the greatest address below the loop header") after measuring it report 25
; cycles where the machine took 1015, on a roll_free:true verdict. The replacement takes
; the counter's entry range from the abstract state of EVERY predecessor of the header
; except the back edge, and returns 0 — stays unbounded — when any is unknown.
;
; Merging two banks into one node set is exactly the condition that breaks that:
;   - if the predecessor scan uses intra-bank successors only, the header below loses
;     its ONLY non-back-edge predecessor (which lives in bank 0), the predecessor set
;     silently becomes just the back edge, and maximising over an INCOMPLETE set
;     under-approximates the entry value, hence the trip count, hence the worst case;
;   - if an address-order filter is applied across banks, it compares $FF02 in bank 0
;     with $FF05 in bank 1 as though they were one program, and addresses in different
;     banks have no order at all.
;
; So this ROM has exactly one job: its crossing region must come back BOUNDED with a
; worst case at or above what the emulator measures. A regression to either shape above
; makes the bound vanish (unbounded) or fall below the machine, and both are visible.
;
;   bank 0 $FF00  ldx #5        <-- the counter's ONLY initialiser
;   bank 0 $FF02  lda $FFF9     hotspot: next fetch is bank 1 $FF05
;   bank 1 $FF05  dex           <-- loop header; its only predecessor is in BANK 0
;   bank 1 $FF06  bne $FF05
;   bank 1 $FF0E  lda $FFF8     hotspot: next fetch is bank 0 $FF11
;   bank 0 $FF11  rts
;
; Runtime evidence that bank 1 really runs the loop: $86 ends every frame at $B1 and
; $87 increments once per frame; both are written only by bank 1.
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
        jsr $FF00           ; counter set here, loop runs in bank 1
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

        ; --- switch zone, bank 0 side ---
        ORG  $0F00
        RORG $FF00
        ldx #5              ; $FF00-01  the loop's ONLY initialiser, in the OTHER bank
        lda $FFF9           ; $FF02-04  hotspot -> next fetch is bank1 $FF05
        ds 12, $EA          ; $FF05-10  bank 1 executes these addresses
        rts                 ; $FF11     bank 1 returns here (bank 0)

        ; --- reset stub, identical in both banks and at the same address ---
        ORG  $0FE0
        RORG $FFE0
        lda $FFF8
        jmp $F000

        ORG  $0FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 1 =================
        ORG  $1000
        RORG $F000
        ds 3, 0             ; bank 1's $F000 is unused

        ; --- switch zone, bank 1 side ---
        ORG  $1F00
        RORG $FF00
        ds 5, $EA           ; $FF00-04  bank 0 executes these addresses
        dex                 ; $FF05     loop header, entered from BANK 0
        bne $FF05           ; $FF06-07
        lda #$B1            ; $FF08-09
        sta $86             ; $FF0A-0B
        inc $87             ; $FF0C-0D
        lda $FFF8           ; $FF0E-10  hotspot -> next fetch is bank0 $FF11

        ; --- reset stub, identical to bank 0's ---
        ORG  $1FE0
        RORG $FFE0
        lda $FFF8
        jmp $F000

        ORG  $1FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0
