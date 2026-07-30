; litmus_bank_unmodelled — F8 (8K, 2 banks): a bank switch that is UNMODELLED BY
; CONSTRUCTION, so the certification gate keeps a witness.
;
; Why it exists. Cross-bank flow is now modelled for the shape the hardware actually
; uses — an instruction whose data access reaches a hotspot continues at the same
; address in the target bank — and with that, all four existing bank ROMs report
; unmodelled_switches:0. The gate that requires that count to be zero before
; certifying would then pass with the gate DELETED. This cartridge keeps it honest:
; its switch can never become modelled, so certified:false stays provable.
;
; The mechanism is an INDIRECT store: `sta (ptr),y` resolves its target through a
; pointer in RAM, which no static address analysis can pin down. Under a mapper that
; publishes bank-switch hotspots that access MAY therefore select a bank, and since no
; address is resolved, no symbol and no landing bank can be named — there is nothing to
; model, only something to refuse. It is the one refusal class that cannot be closed by
; a better decoder.
;
; At run time the pointer aims at RAM $0090, so the store is harmless and the ROM
; renders a normal 262-line frame. The cartridge also performs an ordinary MODELLED
; cross-bank call, so it is a genuine bank-switching program rather than an 8K image
; that never leaves bank 0: $80 ends every frame at $B1 (bank 1 wrote it) and $82
; increments once per frame.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02

ptr     = $84       ; pointer for the unresolvable indirect store

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
        lda #$90            ; ptr -> $0090 (RAM), so the indirect store is harmless
        sta ptr
        lda #0
        sta ptr+1

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
        ; --- the UNMODELLED switch: an indirect store has no resolvable target, so
        ;     under a hotspot-bearing mapper it may select a bank and no landing
        ;     site can be named. This region must be refused, forever.
        ldy #0
        lda #$A5
        sta (ptr),y
        jsr $FF00           ; an ordinary MODELLED cross-bank call
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

        ; --- switch zone, bank 0 side: $FF00 ---
        ORG  $0F00
        RORG $FF00
        lda $FFF9           ; hotspot -> next fetch is bank1 $FF03
        ds 9, $EA           ; $FF03-0B: bank 1 executes these addresses, not these bytes
        rts                 ; $FF0C: bank 1 returns here (bank 0)

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

        ; --- switch zone, bank 1 side: $FF00 ---
        ORG  $1F00
        RORG $FF00
        ds 3, $EA           ; $FF00-02: bank 0 executes these addresses
        lda #$B1            ; $FF03: bank 1's code starts here
        sta $80
        inc $82
        lda $FFF8           ; $FF09: hotspot -> next fetch is bank0 $FF0C (rts)

        ; --- reset stub, identical to bank 0's ---
        ORG  $1FE0
        RORG $FFE0
        lda $FFF8
        jmp $F000

        ORG  $1FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0
