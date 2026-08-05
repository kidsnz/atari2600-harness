; litmus_bank_jmphotspot — FIXTURE. A `jmp` whose TARGET is a bank-switch hotspot.
;
; Source: our own verification evidence (the sole witness for branch C of switchEdges).
; SD-8b judged "the path where a jmp/jsr transfers control into a hotspot was never seen = unsound"
; and fixed that branch — but measured (2026-07-30), its execution count across the 123-image corpus
; was **0** = nobody had ever confirmed the fix. This ROM is its sole witness.
;
; Mechanism: execute `jmp $FFF9` inside bank0's visible kernel. $FFF9 is the F8 BANK1 hotspot,
; so transferring control there means "the instruction fetch at that address itself selects another
; bank" = a transition this analysis does not model. The region must therefore be **refused by name**.
; Derived from: roms/techniques/banked_game.asm (see there for the structure). ORIGINAL HEADER FOLLOWS.
; banked_game — bank-switched game structure template (technique: bankswitching, U-M8, F8 8K)
; The authoring-side counterpart of litmus_bank (F8/F6/F4 verified v0.43.0). A real game's standard three-piece set:
;   ① identical reset stub + vectors in every bank (whichever bank powers up, we land in bank0)
;   ② general-purpose cross-bank trampoline (choreographed at $FF80):
;        bank0 $FF80: lda $FFF9   ; select bank1 -> next fetch $FF83 is bank1
;        bank1 $FF83: jmp B1Work  ; off to bank1's work
;        ...work...   jmp $FF86
;        bank1 $FF86: lda $FFF8   ; back to bank0 -> next fetch $FF89 is bank0
;        bank0 $FF89: rts         ; to the caller (bank0's jsr $FF80)
;      ★ never place an instruction ON the hotspots $FFF8/9: a fetch is a read, so it
;        switches the bank (measured bug: an rts placed at $FFF9 became a reboot loop)
;   ③ data-bank access: copy bank1's level table to a zp buffer during VBLANK (level load)
; Demo: level switch every 120 frames -> bank1's loader writes the 8 PF pattern bytes to $90-97.
; The kernel: bank0 draws PF1 from the buffer as 8 bands.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUPF  = $08
PF1     = $0E

level   = $80       ; 0/1
fc      = $81
buf     = $90       ; level PF pattern x8

; ================= bank 0 (main game) =================
        ORG  $0000
        RORG $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #$5A
        sta COLUPF
        jsr $FF80           ; first level load (level=0)

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
        ; --- level switch every 120 frames -> bank1 loader ---
        inc fc
        lda fc
        cmp #120
        bcc NoSwitch
        lda #0
        sta fc
        lda level
        eor #1
        sta level
        jsr $FF80           ; cross-bank call (bank1 rewrites the buffer)
NoSwitch:
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; ===== visible 192 lines: PF1 from the buffer, 8 bands (24 lines each) =====
        ldx #0              ; band
KBand:  lda buf,x
        sta PF1
        ldy #24
        jmp $FFF9           ; ★PLANTED: control transfer INTO the BANK1 hotspot
KRow:   sta WSYNC
        dey
        bne KRow
        inx
        cpx #8
        bne KBand
        lda #0
        sta PF1

        lda #2
        sta VBLANK
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

        ; --- trampoline -> reset stub (bank0 side, ascending ORG) ---
        ORG  $0F80
        RORG $FF80
        lda $FFF9           ; $FF80-2: select bank1
        ds 6, $EA           ; $FF83-8: bank1 executes these (unused on the bank0 side)
        rts                 ; $FF89: lands here right after returning from bank1

        ORG  $0FE0
        RORG $FFE0
        lda $FFF8           ; reset stub: always boot in bank0
        jmp $F000

        ORG  $0FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 1 (data + loader) =================
        ORG  $1000
        RORG $F000
B1Work:                     ; level load: LvTab[level*8..+7] -> buf
        ldx #0
        lda level
        beq B1Copy
        ldx #8
B1Copy: ldy #0
B1Loop: lda LvTab,x
        sta buf,y
        inx
        iny
        cpy #8
        bne B1Loop
        jmp $FF86           ; back to bank0 (second half of the trampoline)

LvTab:  byte $81,$42,$24,$18,$18,$24,$42,$81   ; level 0 (X shape)
        byte $FF,$7E,$3C,$18,$18,$3C,$7E,$FF   ; level 1 (diamond shape)

        ; --- trampoline -> reset stub (bank1 side, ascending ORG) ---
        ORG  $1F80
        RORG $FF80
        ds 3, $EA           ; $FF80-2: bank0 executes these (unused)
        jmp B1Work          ; $FF83-5: entry
        lda $FFF8           ; $FF86-8: return to bank0
        ds 1, $EA           ; $FF89: bank0's rts (unused on this side)

        ORG  $1FE0
        RORG $FFE0
        lda $FFF8
        jmp $F000

        ORG  $1FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0
