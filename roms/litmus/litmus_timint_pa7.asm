; litmus_timint_pa7 — the D6 in `$93=$C0` is not a measurement, it is a power-on choice.
;
; `docs/fundamentals-audit.md` records the RIOT timer's expiry as *"TIMINT $285 D7 = expired
; ($93=$C0, D7+D6 set)"*, and `roms/litmus/scenarios/timer.json` pins `ram.0x93 == 192`. Both are
; true of this engine. Neither says what D6 IS.
;
; It is not a second expiry bit. The 6532 datasheet (Rockwell, transcribed to the list by Dan Boris
; in 199708) gives the register two independent flags:
;
;	Read and Clear Interrupt Flag ... Bit 7 = Timer IRQ flag
;	                                  Bit 6 = PA7 IRQ flag
;
; and the engine implements exactly that, as two separate booleans:
;
;	timintExpired = 0b10000000        if tmr.expired { v |= timintExpired }
;	timintPA7     = 0b01000000        if tmr.pa7     { v |= timintPA7 }
;
; ★So why is D6 set in a ROM that never touches PA7? Because `Timer.Reset()` opens with
; **`tmr.pa7 = true`**, unconditionally — before the `RandomState` branch, so it is not even part of
; the randomised power-on state. The flag is then cleared by the first access that clears it, and
; never set again unless something actually drives PA7.
;
; ★★That makes half of `$C0` a hardware fact and half an emulator's initial condition, and the
; scenario pins the pair as one number. This ROM separates them, by reading TIMINT **before any
; timer is written and before it could have expired**:
;
;	$80  TIMINT at boot, before anything      — D7 clear (nothing expired), D6 SET (power-on)
;	$81  TIMINT immediately after that read   — the access above cleared the PA7 flag
;	$82  TIMINT after a timer really expires  — D7 set; D6 stays clear, because no edge came
;
; ★★★If $80 comes back $00 one day, the engine has changed its power-on convention and the `$C0`
; in the audit and the `192` in the scenario both become wrong — silently, since a scenario that
; pins a sum cannot say which half moved.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
INTIM   = $0284
TIMINT  = $0285
TIM1T   = $0294

        org $F000
Start:
        sei
        cld
        ldx #$FF
        txs
        ; ★NOTE: no RAM clear before the first read. A `sta $00,x` sweep would touch RIOT mirrors
        ; and could disturb exactly what is being measured. RAM is cleared after the three reads.

        ; --- $80: TIMINT at boot, before any timer is written ----------------------------
        lda TIMINT
        sta $80

        ; --- $81: and again, straight away -----------------------------------------------
        lda TIMINT
        sta $81

        ; --- now clear RAM (the three bytes above are kept) -------------------------------
        ldx #$FF
ClrLoop:
        cpx #$80
        beq ClrDone
        lda #0
        sta $00,x
        dex
        jmp ClrLoop
ClrDone:

        ; --- $82: TIMINT after a real expiry ----------------------------------------------
        lda #$02
        sta TIM1T           ; two ticks, one per cycle
        nop
        nop
        nop
        nop
        ; ★ORDER MATTERS: reading INTIM clears the expired flag, so TIMINT must be read FIRST.
        ;   Measured the wrong way round once — INTIM then TIMINT gave $00 and looked like
        ;   "expiry does not set D7", which is a statement about the read order, not the timer.
        lda TIMINT
        sta $82
        lda INTIM           ; well past expiry
        sta $83
        lda TIMINT
        sta $84             ; and again AFTER the INTIM read: the clear the docs describe

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

        org $FFFC
        .word Start
        .word Start
