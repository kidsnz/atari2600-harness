; litmus_oddsleep — the three ways to spend an ODD number of cycles, and what each one costs.
;
; `ds N, $EA` spends 2N cycles: this repository's only padding idiom, and it can only make EVEN
; delays. Sprite positioning asks for odd ones routinely, and the obvious reach is DASM's `SLEEP`
; macro. **Reading the macro settles what it emits** (dasm 2.20.14.1,
; `machines/atari2600/macro.h`, measured by assembling it):
;
;       SLEEP 2  ->  EA              nop                      2 cy, legal
;       SLEEP 3  ->  04 00           nop $00     ILLEGAL       3 cy
;       SLEEP 4  ->  EA EA                                     4 cy, legal
;       SLEEP 5  ->  04 00 EA        nop $00 + nop             5 cy
;       SLEEP 3  ->  24 00           bit $00     legal         3 cy   (-DNO_ILLEGAL_OPCODES=1)
;
; Two things fall out, and the second is the one nobody says out loud.
;
; **1.** Odd SLEEP emits an illegal opcode by default. That is the archive's complaint — Dennis
; Debro, 200401: *"If you're using the SLEEP macro in macro.h and have not turned off illegal
; opcodes, then when your SLEEP value is >= 3 you're using `nop 0`."* Eckhard Stolberg, 200403:
; *"I think the SLEEP macro uses undocumented opcodes in it's default state. StellaX doesn't
; support these."* Kirk Israel asked in the same thread, *"though if it's a multiple of 2, isn't
; SLEEP 'safe' re: undocumented opcodes?"* — **and nobody answered him.** The assembly above is the
; answer: yes, even is safe, odd is not.
;
; **2.** `NO_ILLEGAL_OPCODES` does NOT make odd SLEEP safe. It swaps `nop $00` for `bit $00`, which
; is a legal opcode that **reads the same address**. $00 has A6 and A7 low, so on a 3F/X07 cart it
; is a bankswitch hotspot — the exact pattern `check_traps.py` warns about. The switch fixes the
; opcode and leaves the trap. Neither branch is safe on a bankswitched cart, and both are invisible
; to a source-level grep because the bytes only exist after macro expansion.
;
; So this ROM measures the third option, proposed by the distillation from Jim Nitchals, 199704:
; *"if you need to delay for 7 cycles, a PHP/PLP is a code-compact way to do it."* PHP+PLP is
; 3+4 = 7 cycles in two bytes, touches no address outside the stack, and PLP puts the flags back.
;
;       $80  cycles for `nop $00`   (want 3)
;       $81  cycles for `bit $00`   (want 3)
;       $82  cycles for `php`       (want 3)
;       $83  cycles for `plp`       (want 4)
;       $84  cycles for `php/plp`   (want 7 — the odd delay, legal, no address touched)
;       $85  flags after php/plp round-trip, want the same byte that went in ($B5)
;       $86  1 if A, X and Y survived php/plp unchanged
;       $87  cycles for `ds 2,$EA`  (want 4 — the even baseline this repository already uses)
;
; ★The overhead of the measurement itself is NOT assumed: $8F records an EMPTY measurement, so
; every figure above is read as (value - $8F). A test that subtracts a hard-coded constant would be
; asserting the harness's own timing rather than the instruction's.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
INTIM   = $0284
TIM1T   = $0294

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

        ; --- $8F: the empty measurement. Everything else is read against this. ---
        lda #$80
        sta TIM1T
        lda INTIM
        eor #$FF
        sta $8F

        ; --- $80: nop $00  (the default odd-SLEEP byte; illegal opcode, reads $00) ---
        lda #$80
        sta TIM1T
        .byte $04, $00      ; nop $00 @skip-ok (this ROM's subject IS the trap; nothing 3F here)
        lda INTIM
        eor #$FF
        sta $80

        ; --- $81: bit $00  (the NO_ILLEGAL_OPCODES odd-SLEEP byte; legal, STILL reads $00) ---
        lda #$80
        sta TIM1T
        .byte $24, $00      ; bit $00 @skip-ok
        lda INTIM
        eor #$FF
        sta $81

        ; --- $82 / $83 / $84: php, plp, and the pair ---
        lda #$80
        sta TIM1T
        php
        lda INTIM
        eor #$FF
        sta $82
        plp

        lda #$80
        sta TIM1T
        plp
        lda INTIM
        eor #$FF
        sta $83
        php                 ; keep the stack balanced

        lda #$80
        sta TIM1T
        php
        plp
        lda INTIM
        eor #$FF
        sta $84

        ; --- $87: the even baseline this repository already uses ---
        lda #$80
        sta TIM1T
        ds 2, $EA
        lda INTIM
        eor #$FF
        sta $87

        ; --- $85: does plp put every flag back? Load a known status and round-trip it. ---
        ; $B5 = N V - B - Z C  (N=1 V=0 B=1 D=0 I=1 Z=0 C=1, and bit5 always reads 1)
        lda #$B5
        pha
        plp                 ; status := $B5
        php                 ; and straight back onto the stack
        plp                 ; round-trip
        php
        pla
        sta $85

        ; --- $86: do php/plp leave A, X and Y alone? ---
        lda #$5A
        ldx #$3C
        ldy #$C3
        php
        plp
        cmp #$5A
        bne RegsBad
        cpx #$3C
        bne RegsBad
        cpy #$C3
        bne RegsBad
        lda #1
        sta $86
        jmp StackTIA        ; ★NOT `jmp Frame` — that skipped everything below it. Twice today
RegsBad:                    ;   a success-path jump flew over a later measurement and the section
        lda #0              ;   read as zero; both times the zero looked like a result.
        sta $86
StackTIA:

        ; --- $88 / $89: the two uses of PHP are MUTUALLY EXCLUSIVE in one kernel ------------
        ; `docs/techniques/missiles-bullets.md:67` points SP at the ENAM mirror so that `PHP`
        ; *writes a TIA register*: "[$011D]=ENAM0, [$011E]=ENAM1, [$011F]=ENABL. Point SP at the
        ; ENAM mirror and `PHP` writes the ..." — the Z flag lands in D1 and lights the missile on
        ; its row. Inside a kernel doing that, spending 7 cycles as PHP/PLP does not spend 7
        ; cycles: it fires the missile. Measure it rather than assert it.
        ;   $88 = ENAM0 seen by the TIA after a PHP with SP at the ENAM0 mirror and Z SET
        ;   $89 = the same with Z CLEAR
        ; The test reads ENAM0 through the emulator; these bytes only record what the ROM asked for.
        ldx #$00
        stx $1D             ; ENAM0 := 0  (missile off)  @rom-write-ok
        ldx #$1D
        txs                 ; SP := $1D -> the next push lands on [$011D] = ENAM0
        lda #$02            ; D1 set = the bit ENAM0 reads
        pha                 ; **a plain stack push, straight into ENAM0**
        ldx #$FF
        txs                 ; put the stack back before anything else runs
        lda #1
        sta $88

        ; --- $89: the same thing with PHP, which is what the delay idiom would do -------------
        ldx #$00
        stx $1D             ; @rom-write-ok
        ; ★Pushes DESCEND, which is why the technique doc writes the ladder as `SP $1E->$1D`:
        ;   start at $1E and one PHP lands on ENAM1, the next on ENAM0.
        ldx #$1E
        txs
        lda #$00
        cmp #$00            ; Z=1, C=1  -> the pushed status has D1 set
        php                 ; [$011E] = ENAM1
        php                 ; [$011D] = ENAM0
        tsx                 ; witness: SP must have descended twice, $1E -> $1C
        stx $89
        ldx #$FF
        txs

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
        lda $84
        sta COLUBK
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
