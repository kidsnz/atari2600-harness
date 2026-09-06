; litmus_delaytable — the 1998 "shortest code for N cycles" table, measured in both currencies.
;
; Andrew Davie posted a delay table 〔stella-list `199805/msg00090`, 1998-05-09 11:07:03〕 under a
; constraint that is the whole point of it:
;
;       I thought I'd go one step further... the shortest code size I know for a particular delay
;       ... but these delays are designed to be NON-destructive of memory. Ie: they have no effect
;       other than delay or the accumulator and/or flags.
;
; He closed it with two open questions — "Can anyone better the byte counts? Any comments on the
; danger of 'writing' to ROM?" — and then, EIGHT MINUTES LATER 〔`199805/msg00091`, 11:15:37〕,
; corrected one of his own rows:
;
;       >5@2 ... STA $8000,X ...
;       That should be 5@3.  I can't figure a 2 byte non-destructive 5 cycle delay :(
;
; ★He fixed the row he was looking at and missed the other one containing the same instruction.
; `11@4` is `STA $8000,X` + `LDA ($80,X)` = 3 bytes + 2 bytes. It has stood as `11@4` in the
; archive ever since. This ROM measures every row so the table can be republished as measured
; rather than as remembered.
;
; ★★And it answers his second question, which the thread never did. On a 2600 the address bus is
; 13 bits and A12 selects the cartridge, so `$8000` is not ROM and never was: it folds to `$0000`,
; the TIA. `STA $8000,X` writes a TIA REGISTER chosen by X. The last section makes that visible
; instead of arguing it — with X = $09 the "harmless write to ROM" sets COLUBK.
;
; Every timing row runs with X = $2C so the folded address is CXCLR, which is why the measurement
; survives its own probe: X = $02 would fold to WSYNC and halt the CPU until the end of the line,
; which is a delay of a completely different kind. The indirect pointer therefore lives at
; $80 + $2C = $AC.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

XSAFE   = $2C               ; folds to CXCLR — see header
PTR     = $AC               ; $80 + XSAFE, the pointer (LDA ($80,X) reads it)

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

        lda #$00            ; the indirect pointer -> $F000, a plain ROM read
        sta PTR
        lda #$F0
        sta PTR+1

        ldx #XSAFE          ; X is set ONCE and never inside a measured interval

; ---- the table. Each row is bracketed by D<nn>_S / D<nn>_E and nothing else. ----
D01_S:  nop
D01_E:
D02_S:  lda $80
D02_E:
D03_S:  nop
        nop
D03_E:
D04_S:  sta $8000,x
D04_E:
D05_S:  lda ($80,x)
D05_E:
D06_S:  pha
        pla
D06_E:
D07_S:  rol $8000,x
D07_E:
D08_S:  lda ($80,x)
        nop
D08_E:
D09_S:  pha
        pla
        nop
D09_E:
D10_S:  lda ($80,x)
        lda $80
D10_E:
D11_S:  rol $80
        ror $80
D11_E:
D12_S:  sta $8000,x
        lda ($80,x)
D12_E:
D13_S:  jsr JustRts
D13_E:
D14_S:  lda ($80,x)
        lda ($80,x)
D14_E:


; ---- what the 2004 consolidation added 〔`200404/msg00246`, Paul Slocum, quoting
; ---- Christopher Tumbler / Chris Wilkson / Andrew Davie, under the heading "Todo: Verify Andrew's" ----
D15_S:  lda $80,x           ; ★+1 cycle for ZERO extra bytes, against D02's `lda $80`
D15_E:
D16_S:  lda.w $80           ; +1 cycle for +1 byte (zp forced to absolute)
D16_E:
D17_S:  dc.b $04,$80        ; `dop $80` — ★ILLEGAL opcode (double NOP). Emitted as bytes because
D17_E:                      ; ★DASM will not assemble it, and measured here as an EMULATOR fact.
D18_S:  dec $2D             ; ★the 2004 answer to Davie's 1998 "I can't figure a 2 byte
D18_E:                      ; ★non-destructive 5 cycle delay :(" — the instruction IS destructive;
D19_S:  dec $2D             ; ★$2D is simply not decoded, so there is nothing there to destroy.
        dec $2D
D19_E:
D20_S:  pha
D20_E:  pla                 ; (D20 measures the push alone; the pull is D21 and also restores SP)
D21_S:  pha
D21_E2: pla
D21_E:

; ---- the 2004 claim itself: "locations $2D-$3F do nothin and aren't decoded" ----
; A WRITE claim, which is a different axis from the read folding measured earlier. The test
; snapshots every write-only TIA register around this block and asserts nothing moved.
Undec_S:
        lda #$FF
        sta $2D
        sta $2E
        sta $2F
        sta $30
        sta $35
        sta $3A
        sta $3F
Undec_E:

; ---- Davie's unanswered question, answered with a pixel ----
; "Any comments on the danger of 'writing' to ROM?"  There is no ROM at $8000 on this machine.
Land_S: ldx #COLUBK
        lda #$44
        sta $8000,x         ; ★folds to TIA $09 = COLUBK. The background changes colour.
Land_E: ldx #XSAFE

Frame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ldy #37
VB:     sta WSYNC
        dey
        bne VB
        lda #0
        sta VBLANK
        ldy #192
Vis:    sta WSYNC
        dey
        bne Vis
        lda #2
        sta VBLANK
        ldy #30
OS:     sta WSYNC
        dey
        bne OS
        jmp Frame

JustRts:
        rts

        org $FFFC
        .word Start
        .word Start
