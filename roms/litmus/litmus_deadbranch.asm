; litmus_deadbranch — a branch whose flag is already decided has ONE successor.
;
; `collectRegion` walked BOTH arms of every branch, and `longest` costed both. That
; is right when the flag is unknown and wrong when it is not — and when the dead arm
; happens to fall into a DATA TABLE, the walk decodes the table as instructions.
;
; Found on the user's own ROM. `sandbox/practice/pizza-boy-tokyo/build/pizza_boy.asm`
; has, at line 673:
;
;       .sxZero:
;               lda #0          ; Z := 1
;               sta Dx          ; a store leaves the flags alone
;               beq .cexit      ; therefore ALWAYS taken
;       Alley3A:
;               .byte 28,28,28,28,28,28,28,0,0,36,...
;
; The fall-through cannot happen, but collection took it anyway, decoded the snap
; table, and hit the `$00` at $F490 — which is a byte of level data and decodes as
; BRK. The region was refused for "BRK in region": an instruction the machine never
; executes, at an address that holds graphics. That refusal is what made the
; project's own `phase0` scenario FAIL.
;
; `refineBranch` — the same test the abstract interpreter has always applied in
; `absSuccessors` — settles it. It prunes only when the flag is KNOWN, so this
; removes paths the machine provably cannot take and no others.
;
; THE ROWS:
;
;   DeadRow    the shape above: a decided `beq` followed immediately by data whose
;              first byte is $00. Must be BOUNDED. Before the fix it is refused for
;              "BRK in region".
;
; THE CONTROLS:
;
;   LiveRow    a branch on a flag the analysis CANNOT decide (it reads SWCHB), also
;              followed by the same kind of data. Both arms are real, so the walk
;              must still take both — and the fall-through still reaches the data,
;              so this row must STAY refused. If it starts passing, the prune is
;              firing on unknown flags and is removing paths the machine can take,
;              which is an under-approximation.
;   PlainRow   an ordinary decided branch with CODE after it rather than data. It
;              was bounded before and must stay bounded at the same number: the
;              prune must not change what it costs, only which arms exist.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
SWCHB   = $0282

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

Main:
; --- VSYNC: 3 lines ---
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 37 lines ---
        lda #2
        sta VBLANK
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

; --- Visible ---
; THE ROW: Z is 1 by construction, so the `beq` is always taken and the table below
; is never reached. Walking the fall-through decodes `$00` as BRK.
DeadRow:
        sta WSYNC
        lda #0          ; Z := 1
        sta $80         ; a store does not touch Z
        beq DeadJoin    ; always taken
DeadData:
        .byte 28,28,28,0,0,36,36,36,68,68,0,0,76,76,76,108
DeadJoin:
        lda #$0E
        sta COLUBK

; CONTROL 1 — the same shape on a flag the analysis cannot decide. Both arms are
; live, the fall-through really does reach the data, and the region must STAY
; refused. A prune that fired here would be removing a path the machine can take.
LiveRow:
        sta WSYNC
        lda SWCHB
        and #$02
        beq LiveJoin    ; genuinely either way
LiveData:
        .byte 12,12,12,0,0,20,20,20,52,52,0,0,60,60,60,92
LiveJoin:
        lda #$00
        sta COLUBK

; CONTROL 2 — a decided branch with CODE after it. Bounded before and after, at the
; same cost: the prune changes which arms exist, not what an arm costs.
PlainRow:
        sta WSYNC
        lda #0
        sta $81
        beq PlainJoin
        nop
        nop
        nop
PlainJoin:
        lda #$00
        sta COLUBK

        ldx #187
Fill:   sta WSYNC
        dex
        bne Fill

; --- Overscan: 30 lines ---
        sta WSYNC
        lda #2
        sta VBLANK
        ldx #29
OS:     sta WSYNC
        dex
        bne OS

        jmp Main

        org $FFFC
        .word Reset
        .word Reset
