; litmus_jmptable_delay — a delay table with SINGLE-CYCLE resolution, and which way it runs.
;
; This repository already knows that spending an ODD number of cycles is awkward: `ds N,$EA` can
; only make even delays, and DASM's `SLEEP <odd>` emits `nop $00` (illegal, and a bankswitch
; hotspot) or `bit $00` under NO_ILLEGAL_OPCODES (legal, same address) — measured in
; `litmus_oddsleep.asm`. The archive has a third answer, older and better, and it was published
; with an open question attached.
;
; Jim Nitchals, 1998-03-18, subject "[stella] Variable cycle delays" 〔stella-list
; `199803/msg00160`〕:
;
;       Here's a way to implement single cycle resolution without the use of the carry flag
;       (which adds overhead in the setup and at the end):
;               sta indjmp
;               jmp (indjmp)
;       JumpTable:
;               dc.b $C9   ; opcode: CMP immediate (4 cycles: uses the $C5, executes the NOP below.)
;               dc.b $C5   ; opcode: CMP zero page (3 cycles, uses up the NOP as a destination
;                          ;   address of $EA)
;               nop        ; opcode: NOP (2 cycles by itself)
;
; The mechanism is that each opcode EATS the byte after it. Land on the last `$C9` and it takes the
; `$C5` as its immediate operand (2 cycles) and then runs the `nop` (2) — four. Land on the `$C5`
; and it takes the `$EA` as a zero-page ADDRESS (3). Land on the `nop` and it is two. Step back one
; more byte and the whole thing re-phases, which is where the odd counts come from.
;
; ★And the open question. Erik Mooney replied the same day 〔`199803/msg00161`〕:
;
;       That's... incredibly wizardly.  But isn't something backwards?  The way I see it,
;       won't a larger value in the accumulator cause it to jump farther into the table,
;       skipping more instructions and delaying fewer cycles?
;
; ★★Chris Wilkson answered him the same day 〔`199803/msg00164`〕: *"Yeah, but mine was like that
; too...the delay equaled the max count minus the accum[ulator]"* — so two people had built this
; independently by the time it was posted, and the sense IS inverted. That settles the reading.
; This ROM settles the measurement: it enters the table at five successive offsets and records what
; each one costs.
;
; ★★★Measured 2026-09-06 (`internal/emu/jmptabledelay_test.go`). Subtracting the empty interval and
; the 10 cycles of scaffolding that every entry pays — `jmp (indjmp)` 5 plus `jmp (RetVec)` 5 —
; the five entries cost **6, 5, 4, 3, 2** cycles. Both claims hold exactly: successive entry points
; differ by **one cycle**, and the deeper the entry the SHORTER the delay.
;
;       $80  entry at CmpZp-3  (shallowest of the five, so the LONGEST — 6 cycles)
;       $81  entry at CmpZp-2  (5)
;       $82  entry at CmpZp-1  (4)
;       $83  entry at CmpZp    ($C5, CMP zero page — 3)
;       $84  entry at NopByte  ($EA, the bare nop — 2, the SHORTEST)
;       $8F  an empty interval, so every figure above is read as (value - $8F)
;
; ★★★The five differ only in the value written to `indjmp`; every other instruction is identical,
; so the differences are the table's and not the scaffolding's.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
INTIM   = $0284
TIM1T   = $0294

indjmp  = $90               ; two bytes of RAM for the indirect vector
RetVec  = $92               ; ★and two more for the return vector: each measurement sets its own,
                            ; ★★or every entry lands on the first one and the ROM loops there

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

        lda #>JumpTable     ; the table is page-aligned, so only the low byte ever changes
        sta indjmp+1

        ; --- $8F: the empty interval ---
        lda #$80
        sta TIM1T
        lda INTIM
        eor #$FF
        sta $8F

        ; --- $80: enter four bytes before the tail ---
        lda #<CmpZp-3
        sta indjmp
        lda #<Land0
        sta RetVec
        lda #>Land0
        sta RetVec+1
        lda #$80
        sta TIM1T
        jmp (indjmp)
Land0:  lda INTIM
        eor #$FF
        sta $80

        ; --- $81 ---
        lda #<CmpZp-2
        sta indjmp
        lda #<Land1
        sta RetVec
        lda #>Land1
        sta RetVec+1
        lda #$80
        sta TIM1T
        jmp (indjmp)
Land1:  lda INTIM
        eor #$FF
        sta $81

        ; --- $82 ---
        lda #<CmpZp-1
        sta indjmp
        lda #<Land2
        sta RetVec
        lda #>Land2
        sta RetVec+1
        lda #$80
        sta TIM1T
        jmp (indjmp)
Land2:  lda INTIM
        eor #$FF
        sta $82

        ; --- $83: the $C5, CMP zero page ---
        lda #<CmpZp
        sta indjmp
        lda #<Land3
        sta RetVec
        lda #>Land3
        sta RetVec+1
        lda #$80
        sta TIM1T
        jmp (indjmp)
Land3:  lda INTIM
        eor #$FF
        sta $83

        ; --- $84: the bare nop ---
        lda #<NopByte
        sta indjmp
        lda #<Land4
        sta RetVec
        lda #>Land4
        sta RetVec+1
        lda #$80
        sta TIM1T
        jmp (indjmp)
Land4:  lda INTIM
        eor #$FF
        sta $84

Hold:   lda #2
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
        ldx #192
Pic:    sta WSYNC
        dex
        bne Pic
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp Hold

; ★The table itself. Every entry falls through into `Done`, which is the single landing point —
; the five measurements above differ ONLY in where they enter, never in where they leave.
        align 256
; ★The table is sixteen `$C9` deep for one reason: the entry points must all sit in the SAME PAGE.
; The first version had three, so `NopByte-4` fell to $F0FF while `indjmp+1` still said $F1 — the
; jump went to $F1FF, the ROM ran off into unwritten bytes, and every measurement read zero. A
; page-aligned table whose entries reach back past its own start is not page-aligned for those
; entries. Measured the hard way.
JumpTable:
        ds 16, $C9          ; CMP #imm — each one swallows the byte after it
CmpZp:  .byte $C5           ; CMP zp — swallows the $EA below as its ADDRESS
NopByte:
        .byte $EA           ; NOP
        jmp (RetVec)

        org $FFFC
        .word Start
        .word Start
