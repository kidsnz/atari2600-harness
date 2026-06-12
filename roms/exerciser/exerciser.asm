; exerciser — 全能力統合のショーケース ROM（Road to 1.0）
; M1: F8 2バンク骨格＋シーンディスパッチ＋fire 切替（v0.56.0 検証済）。
; M2: S0=タイトル（bank1）: 48px "EXRCSR" ロゴ（6書換 kernel・テーブル参照版）＋6桁 BCD スコア。
;
; シーン契約: 「ちょうど 192 scanline を消費するサブルーチン」。フレーム骨格は bank0 が所有。
; シーン: 0=Title(bank1) / 1=Blue placeholder(bank0)。fire 押下エッジで前進・wrap。
;
; RAM マップ:
;  グローバル: $80=scene  $81=prevFire  $82=frameCt  $90-$92=スコアBCD(6桁,LE)  $9E/$9F=実行センチネル
;  Title ローカル: $A0=row  $A1-$A3=t3,t4,t5  $A4-$A6=t0,t1,t2  $A8-$B3=p0..p5(各2B, digit ptr)
;
; 48px ロゴ kernel（litmus_48px6 の検証済み振付のテーブル参照版・1ライン/行・行倍化テーブルで16行）:
;  tail(前行末): B5→t5, B0→GRP0, B1→A, WSYNC
;  head: sta GRP1(B1)@3 → B2→GRP0@10 → B3/B4/B5→A/X/Y@21 → SLEEP10 → timed store 完了 34/37/40/43
; スコア kernel（2ライン交互=stage/draw, 縞表示）:
;  stage 行: (p0..p5),y から t0..t5 へ（76cy 内で余裕）
;  draw 行: zp 21cy プリロード → SLEEP4 → timed 34/37/40/43（litmus_48px6 と同一）
; M3 実機裏取り済（v0.58.0）: 非対称山並み2種（PF1 早書き≤27/右38=pf_async 窓）＋6ゾーン12スプライト・ドリフト
;（4行分割で各行≤76cy）＋地面。シーン進入初期化=lastScene 方式。264→262 は assert_line_budget 系の勘定で調整。
; 実機裏取り済（Gopher2600, v0.57.0）: 48px "EXRCSR" 可読（P0=24/P1=32, NUSIZ=3, VDEL, timed 34/37/40/43）。
; 6桁BCDスコアがフレーム毎 +1（frame123 で "000122" 表示・$90-92 BCD 一致）。スコアは stage/draw 交互（縞）。
; 修正史: ①ロゴ表 align 256（abs,y ページ跨ぎ+1cy のシアー） ②ループ折返し jmp を WSYNC 後 0-2cy に置く +3 シフト head
; ③stage 行頭でスプライト消灯（VDEL 影の残骸） ④GLYPH を逆順格納（row 降順描画で正立）。回帰固定=scenarios/m2_title.json。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
NUSIZ1  = $05
COLUP0  = $06
COLUP1  = $07
COLUBK  = $09
RESP0   = $10
RESP1   = $11
GRP0    = $1B
GRP1    = $1C
HMP0    = $20
HMP1    = $21
HMOVE   = $2A
HMCLR   = $2B
VDELP0  = $25
VDELP1  = $26
INPT4   = $3C
INPT0   = $38
AUDC0   = $15
AUDF0   = $17
AUDV0   = $19
AUDC1   = $16
AUDF1   = $18
AUDV1   = $1A
SWCHA   = $0280
ENAM0   = $1D
ENABL   = $1F
RESM0   = $12
RESBL   = $14
HMM0    = $22
PF0     = $0D
PF2     = $0F
CXCLR   = $2C
CTRLPF  = $0A
COLUPF  = $08
PF1     = $0E

NSCENES = 5

scene    = $80
prevFire = $81
frameCt  = $82
lastScene = $83     ; シーン進入検出（初期化 1 回/進入）
m0idx    = $84      ; 音楽: ch0 ノート index
m0dur    = $85      ; 音楽: ch0 残フレーム
m1idx    = $86
m1dur    = $87
score0   = $90      ; BCD 下2桁
score1   = $91
score2   = $92      ; BCD 上2桁
sent0    = $9E
sent1    = $9F
row      = $A0
t3       = $A1
t4       = $A2
t5       = $A3
t0       = $A4
t1       = $A5
t2       = $A6
p0       = $A8      ; digit ptr ×6（各2B: $A8,$AA,$AC,$AE,$B0,$B2）
p1       = $AA
p2       = $AC
p3       = $AE
p4       = $B0
p5       = $B2
; Zone シーン・ローカル（Title と排他オーバーレイ）
zx0      = $A4      ; P0 X ×6（$A4-$A9）
zx1      = $AA      ; P1 X ×6（$AA-$AF）
; Paddle シーン・ローカル
pdlCnt   = $B6      ; 今フレームの充電カウント
pdlPos   = $B7      ; 前フレーム確定のカーソル X（描画用）
pdlDone  = $B8
; Gradient シーン・ローカル
sfxTmr   = $B9      ; SFX 減衰タイマ
; Procedural シーン・ローカル
worldSeed = $BA     ; 現在の世界シード（64 フレーム毎に進む）
rnd      = $BB      ; 行毎 LFSR 作業用
; Playground シーン・ローカル（排他オーバーレイ）
px       = $A4      ; P0 X（10-140）
py       = $A5      ; P0 Y（10-170）
mx       = $A6      ; missile X
mAct     = $A7      ; missile 飛行中フラグ
hitCol   = $A9      ; 衝突フィードバック色
lineCt   = $B4      ; kernel 行カウンタ

; ================= bank 0: フレームワーク＋S1 =================
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
        sta $2C             ; CXCLR
        lda #$FF
        sta lastScene       ; 全シーンの進入初期化を初回必ず発火させる

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
        ; --- 入力: fire 押下エッジで scene++（wrap）---
        lda INPT4
        and #$80
        tay
        cmp prevFire
        beq NoEdge
        cmp #0
        bne NoEdge
        inc scene
        lda scene
        cmp #NSCENES
        bcc NoWrap
        lda #0
        sta scene
NoWrap:
NoEdge: sty prevFire
        inc frameCt
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; --- シーンディスパッチ ---
        lda scene
        beq DoTitle
        cmp #1
        beq DoZone
        cmp #2
        beq DoPlay
        cmp #3
        beq DoGrad
        jsr SceneProc       ; scene 4（bank0）
        jmp AfterScene
DoGrad: jsr SceneGrad       ; scene 3（bank0）
        jmp AfterScene
DoPlay: jsr ScenePlay       ; scene 2（bank0）
        jmp AfterScene
DoZone: jsr SceneZone       ; scene 1（bank0）
        jmp AfterScene
DoTitle:
        jsr CallB1Scene     ; scene 0 = Title（bank1）
AfterScene:

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame


; --- S1: ゾーン風景（bank0）---
; 合成: 非対称PF山並み(48行, pf_async の窓) + 6ゾーン×P0/P1=12個の動くスプライト(96行,
; techniques/zone_multiplex の検証済み構造) + 地面(45行)。先頭 3 行=進入初期化 or ドリフト更新。
SceneZone:
        lda #$C2
        sta $9D             ; 実行センチネル
        lda #0
        sta COLUBK
        sta CTRLPF          ; repeat
        lda #$0E
        sta COLUP0
        lda #$48
        sta COLUP1
        lda #0
        sta NUSIZ0
        sta NUSIZ1
        sta VDELP0
        sta VDELP1
        lda #$D6
        sta COLUPF          ; 山の色（黄緑）
        sta WSYNC           ; 構成行を閉じる（1）
        ; --- 行2-4: 進入初期化（X 初期値コピー）or ドリフト（P0右/P1左, and #$7F）---
        lda lastScene
        cmp #1
        beq ZDrift
        ; 初期化パス（4 行）＋音楽ミュート
        lda #0
        sta AUDV0
        sta AUDV1
        ldx #5
ZI1:    lda ZoneXInit0,x
        sta zx0,x
        dex
        bpl ZI1
        sta WSYNC           ; 2
        ldx #5
ZI2:    lda ZoneXInit1,x
        sta zx1,x
        dex
        bpl ZI2
        sta WSYNC           ; 3
        lda #1
        sta lastScene
        sta WSYNC           ; 4
        sta WSYNC           ; 5
        jmp ZMountains
ZDrift: ; ドリフトパス（4 行: 3ゾーンずつ＝各行 ~65cy で 76 内）
        ldx #5
ZD1:    lda zx0,x
        clc
        adc #1
        and #$7F
        sta zx0,x
        dex
        cpx #2
        bne ZD1
        sta WSYNC           ; 2
ZD2:    lda zx0,x
        clc
        adc #1
        and #$7F
        sta zx0,x
        dex
        bpl ZD2
        sta WSYNC           ; 3
        ldx #5
ZD3:    lda zx1,x
        sec
        sbc #1
        and #$7F
        sta zx1,x
        dex
        cpx #2
        bne ZD3
        sta WSYNC           ; 4
ZD4:    lda zx1,x
        sec
        sbc #1
        and #$7F
        sta zx1,x
        dex
        bpl ZD4
        sta WSYNC           ; 5
ZMountains:
        ; --- 山並み 48 行: 非対称 PF1（左=MtnL 早書き≤27 / 右=MtnR cyc38 書き=窓37-53）---
        ldy #0
ZMt:    sta WSYNC
        lda MtnL,y          ; 4
        sta PF1             ; 7   左 PF1（窓 ≤27 ✓）
        lda MtnR,y          ; 11
        ds 12, $EA          ; 35
        sta PF1             ; 38  右 PF1（窓 37-53 ✓）
        iny
        cpy #48
        bne ZMt
        lda #0
        sta PF1
        ; --- ゾーン 6×16 行（zone_multiplex の検証済み構造）---
        ldx #0
ZLoop:  sta WSYNC
        lda zx0,x
        sec
ZDv0:   sbc #15
        bcs ZDv0
        tay
        lda ZHmove,y
        sta HMP0
        sta RESP0
        sta WSYNC
        lda zx1,x
        sec
ZDv1:   sbc #15
        bcs ZDv1
        tay
        lda ZHmove,y
        sta HMP1
        sta RESP1
        sta WSYNC
        lda ZoneBG,x
        sta COLUBK
        sta HMOVE
        ldy #0
ZSpr:   sta WSYNC
        lda ZSprite,y
        sta GRP0
        sta GRP1
        iny
        cpy #8
        bne ZSpr
        lda #0
        sta GRP0
        sta GRP1
        ldy #5
ZPad:   sta WSYNC
        dey
        bne ZPad
        inx
        cpx #6
        bne ZLoop
        ; --- 地面 43 行（1+4+48+96+43 = 192）---
        lda #$E4
        sta COLUBK
        ldy #43
ZGnd:   sta WSYNC
        dey
        bne ZGnd
        lda #0
        sta COLUBK
        sta HMCLR
        rts

ZoneXInit0:
        .byte 20, 50, 90, 120, 110, 70
ZoneXInit1:
        .byte 100, 70, 30, 60, 95, 120
ZoneBG:
        .byte $84, $94, $A4, $B4, $C4, $24
ZSprite:
        .byte $18,$3C,$7E,$FF,$FF,$7E,$3C,$18
MtnL:   ; 山シルエット左（PF1, きっかり 48 エントリ）
        ds 12, $00
        .byte $01,$01,$03,$03,$07,$07,$0F,$0F,$1F,$1F,$3F,$3F
        .byte $7F,$7F,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF
        ds 12, $FF
MtnR:   ; 山シルエット右（別形状=非対称の証明・きっかり 48 エントリ）
        ds 6, $00
        .byte $80,$80,$C0,$C0,$E0,$E0,$F0,$F0,$F8,$F8,$FC,$FC,$FE,$FE
        .byte $FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF
        ds 14, $FF


; --- S2: 操作プレイグラウンド（bank0）---
; ジョイスティックで P0 移動（SWCHA, litmus_input）/ ミサイル自動発射（32フレーム毎, 右へ 1px/フレーム=HMOVE ドリフト）
; / PF 側壁＋中央 ball / 衝突ラッチで P0 色が変わる（litmus_collide_all の意味づけ）。CXCLR 毎フレーム。
ScenePlay:
        lda #$D4
        sta $9C             ; 実行センチネル
        lda lastScene
        cmp #2
        beq PNoInit
        ; 進入初期化
        lda #70
        sta px
        lda #80
        sta py
        lda #0
        sta mAct
        sta AUDV0
        sta AUDV1
        lda #2
        sta lastScene
PNoInit:
        ; 構成: PF 側壁（reflect で左右対称・PF0 のみ）, ball 中央
        lda #$F0
        sta PF0
        lda #0
        sta PF1
        sta PF2
        lda #1
        sta CTRLPF          ; reflect
        lda #$3A
        sta COLUPF
        lda #0
        sta COLUBK
        sta NUSIZ0
        lda #$1E
        sta COLUP1
        lda #0
        sta GRP1
        sta WSYNC           ; 1
        ; --- 入力＋移動（行2）---
        lda SWCHA
        and #$80            ; right (0=押下)
        bne PNotR
        inc px
PNotR:  lda SWCHA
        and #$40            ; left
        bne PNotL
        dec px
PNotL:  lda SWCHA
        and #$20            ; down
        bne PNotD
        inc py
PNotD:  lda SWCHA
        and #$10            ; up
        bne PNotU
        dec py
PNotU:  sta WSYNC           ; 2
        ; --- クランプ＋衝突フィードバック（行3）---
        lda px
        cmp #10
        bcs PXlo
        lda #10
        sta px
PXlo:   lda px
        cmp #141
        bcc PXhi
        lda #140
        sta px
PXhi:   lda py
        cmp #10
        bcs PYlo
        lda #10
        sta py
PYlo:   lda py
        cmp #170
        bcc PYhi
        lda #169
        sta py
PYhi:   sta WSYNC           ; 3b（クランプ行と衝突行を分離: 合算 >76cy 対策）
        ; 衝突: 前フレームの描画で立ったラッチを「読んでから消す」（読み→CXCLR の順が肝）
        lda #$0E
        sta hitCol
        bit $32             ; CXP0FB: D7=p0-pf（壁）
        bpl PChkBL
        lda #$44
        sta hitCol          ; 赤=壁ヒット
PChkBL: bit $32             ; CXP0FB D6 = p0-bl（ポール）
        bvc PNoBL2
        lda #$86
        sta hitCol          ; 青=ポールヒット（CXP0FB D6 = p0-bl）
PNoBL2: lda hitCol
        sta COLUP0
        sta CXCLR           ; 読了後にクリア（次フレームの描画で再蓄積）
        sta WSYNC           ; 3
        ; --- ミサイル管理（行4）---
        lda mAct
        bne PMfly
        lda frameCt
        and #$1F
        bne PMnone          ; 32フレーム毎に発射
        lda px
        sta mx
        lda #1
        sta mAct
        jmp PMnone
PMfly:  inc mx
        lda mx
        cmp #150
        bcc PMok
        lda #0
        sta mAct
PMok:
PMnone: lda mAct
        asl                 ; 0/2
        sta ENAM0           ; D1
        lda #2
        sta ENABL           ; ball 常時
        sta WSYNC           ; 4
        ; --- 位置決め: P0（行5）, M0（行6）, BL（行7, 中央固定 X=80）, HMOVE（行8）---
        lda px
        sec
PDv0:   sbc #15
        bcs PDv0
        tay
        lda ZHmove,y
        sta HMP0
        sta RESP0
        sta WSYNC           ; 5
        lda mx
        sec
PDv1:   sbc #15
        bcs PDv1
        tay
        lda ZHmove,y
        sta HMM0
        sta RESM0
        sta WSYNC           ; 6
        lda #80
        sec
PDv2:   sbc #15
        bcs PDv2
        tay
        lda ZHmove,y
        sta $24             ; HMBL
        sta RESBL
        sta WSYNC           ; 7
        sta HMOVE
        ds 12, $EA          ; SLEEP 24（HMOVE 後の HMxx 凍結確保）
        sta HMCLR
        sta WSYNC           ; 8
        ; --- kernel 180 行: py ウィンドウで P0 描画 ---
        lda #0
        sta lineCt
PKer:   sta WSYNC
        lda lineCt          ; 3
        sec                 ; 5
        sbc py              ; 8
        cmp #8              ; 10
        bcs PBlank          ; 12
        tay                 ; 14
        lda ZSprite,y       ; 18
        sta GRP0            ; 21
        jmp PNext           ; 24
PBlank: lda #0              ; 15
        sta GRP0            ; 18
        nop
        nop                 ; 22
        nop                 ; 24（分岐を等長化）
PNext:  inc lineCt          ; 29
        lda lineCt          ; 32
        cmp #180            ; 34
        bne PKer            ; 37
        lda #0
        sta GRP0
        sta ENAM0
        sta ENABL
        sta WSYNC           ; 188
        ; --- 残り 1 行（合計 192）---
        sta WSYNC           ; 192
        lda #0
        sta PF0
        sta CTRLPF
        rts

; （パドルシーンは v1.0.1 で撤去: ROM 内の INPT0 読取を Stella が「パドル用 ROM」と自動判別し、
;   パドルが挿さると INPT4 が常時 High＝fire が効かなくなるため。パドル能力の検証は litmus_paddle に現存。）

; --- S4: グラデーション＋SFX（bank0）---
; per-scanline COLUBK 虹（litmus_color 系）＋ 進入時に kick（Buzz AUDC=15, AUDF=30, AUDV 減衰=Slocum レシピ）。
SceneGrad:
        lda #$F8
        sta $9A             ; 実行センチネル
        lda lastScene
        cmp #3
        beq GNoInit
        lda #3
        sta lastScene
        lda #0
        sta AUDV1
        ; SFX: kick 開始
        lda #15
        sta AUDC0
        lda #30
        sta AUDF0
        lda #10
        sta AUDV0
        lda #40
        sta sfxTmr
GNoInit:
        ; 減衰: 4 フレーム毎に AUDV--
        lda sfxTmr
        beq GSilent
        dec sfxTmr
        lda sfxTmr
        and #$03
        bne GVolOk
        lda sfxTmr
        lsr
        lsr
        sta AUDV0
GVolOk: jmp GRun
GSilent:
        lda #0
        sta AUDV0
GRun:   sta WSYNC           ; 1
        ; --- 虹 190 行: COLUBK = 行インデックス（луma 巡回）---
        ldy #0
GKer:   sta WSYNC
        tya                 ; 行 → 色（hue が流れる）
        clc
        adc frameCt         ; フレームでスクロール
        sta COLUBK
        iny
        cpy #190
        bne GKer
        lda #0
        sta COLUBK
        sta WSYNC           ; 192
        rts

; --- S5: 手続き生成（bank0）---
; LFSR（litmus_lfsr で数学検証済みの eor #$8E Galois）で行毎に星空 PF を生成。
; worldSeed を行頭で rnd に再ロード→各行 1 ステップ＝フレーム内は決定的（静止画）。
; 64 フレーム毎に worldSeed を 1 ステップ進める＝世界が周期的に組み変わる（Pitfall/DaveC オマージュ）。
SceneProc:
        lda #$E9
        sta $99             ; 実行センチネル
        lda lastScene
        cmp #4
        beq RNoInit
        lda #4
        sta lastScene
        lda #$2F
        sta worldSeed       ; 初期世界
        lda #0
        sta AUDV0
        sta AUDV1
RNoInit:
        lda frameCt
        and #$3F
        bne RKeep
        ; 64 フレーム毎: 世界を 1 ステップ進める
        lda worldSeed
        lsr
        bcc RNoEor
        eor #$8E
RNoEor: sta worldSeed
RKeep:  lda worldSeed
        sta rnd
        lda #$0E
        sta COLUPF
        lda #0
        sta COLUBK
        sta CTRLPF
        sta PF0
        sta WSYNC           ; 1
        ; --- 星空 190 行: 行毎 LFSR → 疎な PF1/PF2 ---
        ldy #0
RKer:   sta WSYNC
        lda rnd             ; 3
        lsr                 ; 5
        bcc RSk             ; 7
        eor #$8E            ; 9
RSk:    sta rnd             ; 12
        and #$88            ; 14  疎マスク（点をまばらに）
        sta PF1             ; 17
        lda rnd             ; 20
        and #$11            ; 22
        sta PF2             ; 25
        iny
        cpy #190
        bne RKer
        lda #0
        sta PF1
        sta PF2
        rts                 ; 1+190+(framework) = 192 内: 末尾 WSYNC は不要だった

        ; --- HMOVE 表（ページ跨ぎ回避の負インデックス, zone_multiplex と同型）---
        ORG  $0E00
        RORG $FE00
ZHmoveTbl:
        .byte $80,$70,$60,$50,$40,$30,$20,$10,$00,$F0,$E0,$D0,$C0,$B0,$A0,$90
ZHmoveEnd:
ZHmove = ZHmoveEnd - 256

        ; --- 切替ゾーン（bank0 側, $FF00）---
        ORG  $0F00
        RORG $FF00
CallB1Scene:
        lda $FFF9
        ds 6, $EA
        rts                 ; $FF09

        ORG  $0FE0
        RORG $FFE0
        lda $FFF8
        jmp $F000

        ORG  $0FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 1: S0 タイトル =================
        ORG  $1000
        RORG $F000
TitleScene:
        lda #$B1
        sta sent1
        ; 音楽: タイトル進入で初期化（lastScene 方式）
        lda lastScene
        beq TMusOk
        lda #0
        sta lastScene
        sta m0idx
        sta m1idx
        lda #1
        sta m0dur
        sta m1dur
TMusOk:
        ; スプライト構成（48px: NUSIZ 3コピー, P0=24/P1=+8, VDEL on）
        lda #$03
        sta NUSIZ0
        sta NUSIZ1
        lda #1
        sta VDELP0
        sta VDELP1
        lda #$0E
        sta COLUP0
        sta COLUP1
        lda #0
        sta COLUBK

        ; --- 位置決め（2行）: litmus_48px の検証済みレシピ ---
        sta WSYNC
        ds 13, $EA          ; SLEEP 26
        sta RESP0
        sta RESP1
        lda #$10
        sta HMP1
        sta WSYNC
        sta HMOVE
        ds 12, $EA          ; SLEEP 24
        sta HMCLR
        sta WSYNC           ; HMOVE 行をここで閉じる（閉じないと計算と合体して 93cy=2 scanline 跨ぎ→263 行）
        ; ここまで 3 行

        ; --- 計算行 1: BCD スコア +1 ＋ ptr hi 設定 ---
        sed
        clc
        lda score0
        adc #1
        sta score0
        lda score1
        adc #0
        sta score1
        lda score2
        adc #0
        sta score2
        cld
        lda #>Digits
        sta p0+1
        sta p1+1
        sta p2+1
        sta p3+1
        sta p4+1
        sta p5+1
        sta WSYNC           ; 計算行 1 終了（3 行目）

        ; --- 計算行 2: digit ptr lo（nibble*16; 上位 nibble は AND $F0 でそのまま）---
        ; 表示順: p0=score2上位 p1=score2下位 p2=score1上位 ... p5=score0下位
        lda score2
        and #$F0
        sta p0
        lda score2
        and #$0F
        asl
        asl
        asl
        asl
        sta p1
        lda score1
        and #$F0
        sta p2
        lda score1
        and #$0F
        asl
        asl
        asl
        asl
        sta p3
        sta WSYNC           ; 計算行 2（4 行目）
        lda score0
        and #$F0
        sta p4
        lda score0
        and #$0F
        asl
        asl
        asl
        asl
        sta p5
        sta WSYNC           ; 計算行 3（5 行目）

        ; --- 上余白: 35 行（ここまで 5 行 → 40 行）---
        ldy #35
TB1:    sta WSYNC
        dey
        bne TB1

        ; --- ロゴ 16 行（行倍化テーブル, 1ライン kernel）---
        ; ループ構造: tail 末尾で sta WSYNC(74) → 次行頭で jmp(0-2) → head は +3 シフト設計。
        ; プリステージ（row=15 の分）。GRP0=B0 書込は VDEL の影側が 0 のため画面には出ない。
        ldy #15
        sty row
        lda TblR2,y
        sta t5
        lda TblE,y
        sta GRP0            ; B0→P0新
        lda TblX,y          ; B1 を A に
        sta WSYNC
        jmp LogoHead        ; 次行の cycle 0-2 で実行（毎イテレーション同型）
LogoHead:
        sta GRP1            ; 6   B1→P1新（B0→P0影）
        lda TblR1,y         ; 10  B2
        sta GRP0            ; 13  B2→P0新（B1→P1影）
        lda TblC,y          ; 17  B3
        ldx TblS,y          ; 21  B4
        ldy t5              ; 24  B5
        cmp t5              ; 27  SLEEP 7（フラグのみ・A 不変）
        ds 2, $EA           ; 31
        sta GRP1            ; 34  B3→P1新, B2→P0影
        stx GRP0            ; 37  B4→P0新, B3→P1影
        sty GRP1            ; 40  B5→P1新, B4→P0影
        sta GRP0            ; 43  junk,    B5→P1影
        ldy row             ; 46
        dey                 ; 48
        sty row             ; 51
        bmi LogoDone        ; 53（不成立）
        lda TblR2,y         ; 57  次行 B5
        sta t5              ; 60
        lda TblE,y          ; 64  次行 B0
        sta GRP0            ; 67
        lda TblX,y          ; 71  次行 B1（A 保持）
        sta WSYNC           ; 74 → stall → 次行頭の jmp へ
        jmp LogoHead
LogoDone:
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0            ; 影もクリア
        sta WSYNC           ; LogoDone の行を閉じる（17 行目）

        ; --- 中余白: 29 行（ここまで 40+17 → 86 行）---
        ldy #29
TB2:    sta WSYNC
        dey
        bne TB2

        ; --- スコア 32 行（16 行ぶんの行倍化 digit を stage/draw 交互で）---
        ldy #15
        sty row
ScoreLoop:
        ; stage 行: 頭でスプライト消灯（影まで, cycle 11 までに完了=表示前）→ t0..t5 ← (p0..p5),row
        sta WSYNC
        lda #0              ; 2
        sta GRP0            ; 5
        sta GRP1            ; 8   （P0影←0）
        sta GRP0            ; 11  （P1影←0）
        ldy row             ; 14
        lda (p0),y          ; 19
        sta t0              ; 11
        lda (p1),y          ; 16
        sta t1              ; 19
        lda (p2),y          ; 24
        sta t2              ; 27
        lda (p3),y          ; 32
        sta t3              ; 35
        lda (p4),y          ; 40
        sta t4              ; 43
        lda (p5),y          ; 48
        sta t5              ; 51
        ; draw 行: zp プリロード → timed（litmus_48px6 と同一形）
        sta WSYNC
        lda t0              ; 3
        sta GRP0            ; 6   B0
        lda t1              ; 9
        sta GRP1            ; 12  B1（B0→影）
        lda t2              ; 15
        sta GRP0            ; 18  B2（B1→影）
        lda t3              ; 21
        ldx t4              ; 24
        ldy t5              ; 27
        ds 2, $EA           ; 31  SLEEP 4
        sta GRP1            ; 34
        stx GRP0            ; 37
        sty GRP1            ; 40
        sta GRP0            ; 43
        ldy row             ; 46
        dey                 ; 48
        sty row             ; 51
        bpl ScoreLoop       ; 54
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0
        sta WSYNC           ; 閉じ行（86+32+1=119 行目）

        ; --- 音楽 tick（2 行: ch0 / ch1）---
        sta WSYNC
        ; ch0（square リード）
        dec m0dur
        bne TM0ok
        ldy m0idx
        lda Song0,y
        cmp #$FF
        bne TM0note
        ldy #0              ; ループ
        lda Song0,y
TM0note:
        pha
        and #$1F
        sta AUDF0
        pla
        lsr
        lsr
        lsr
        lsr
        lsr
        tax
        lda MusTypes,x
        sta AUDC0
        lda #8
        sta AUDV0
        lda Song0+1,y
        sta m0dur
        iny
        iny
        sty m0idx
TM0ok:  sta WSYNC
        ; ch1（bass）
        dec m1dur
        bne TM1ok
        ldy m1idx
        lda Song1,y
        cmp #$FF
        bne TM1note
        ldy #0
        lda Song1,y
TM1note:
        pha
        and #$1F
        sta AUDF1
        pla
        lsr
        lsr
        lsr
        lsr
        lsr
        tax
        lda MusTypes,x
        sta AUDC1
        lda #8
        sta AUDV1
        lda Song1+1,y
        sta m1dur
        iny
        iny
        sty m1idx
TM1ok:
        ; --- 下余白: 70 行（合計 192）---
        ldy #70
TB3:    sta WSYNC
        dey
        bne TB3
        rts

        ; --- 音楽データ（Sequencer-Kit 互換 noteByte: 上位3bit=type idx, 下位5bit=AUDF / 第2バイト=持続フレーム）---
MusTypes:
        .byte 4, 6, 12, 8   ; idx0=square idx1=bass idx2=lead idx3=noise
Song0:  ; リード（square, type idx0）: AUDF 14,11,9,11
        .byte %00001110, 16
        .byte %00001011, 16
        .byte %00001001, 16
        .byte %00001011, 16
        .byte $FF
Song1:  ; ベース（bass, type idx1=%001xxxxx）: AUDF 31,23 をゆっくり
        .byte %00111111, 32
        .byte %00110111, 32
        .byte $FF

; --- フォント（8×8, 各行を2回置いた 16B/グリフ＝行倍化） ---
        ; ロゴ "EXRCSR": B0=E B1=X B2=R B3=C B4=S B5=R
        MAC GLYPH           ; 8 行を倍化して並べる（kernel は row=15→0 の降順で描く＝最下バイトが画面最上段。
        ; よって行は逆順（{8}が先頭）に格納して、画面では {1}=上段 になる）
        .byte {8},{8},{7},{7},{6},{6},{5},{5},{4},{4},{3},{3},{2},{2},{1},{1}
        ENDM

        align 256
TblE:   GLYPH %01111110,%01100000,%01100000,%01111100,%01100000,%01100000,%01111110,%00000000
TblX:   GLYPH %01100110,%01100110,%00111100,%00011000,%00111100,%01100110,%01100110,%00000000
TblR1:  GLYPH %01111100,%01100110,%01100110,%01111100,%01101100,%01100110,%01100110,%00000000
TblC:   GLYPH %00111100,%01100110,%01100000,%01100000,%01100000,%01100110,%00111100,%00000000
TblS:   GLYPH %00111110,%01100000,%01100000,%00111100,%00000110,%00000110,%01111100,%00000000
TblR2:  GLYPH %01111100,%01100110,%01100110,%01111100,%01101100,%01100110,%01100110,%00000000

        ; 数字 0-9（16B ストライド・ページ内に収める）
        align 256
Digits:
        GLYPH %00111100,%01100110,%01101110,%01110110,%01100110,%01100110,%00111100,%00000000 ; 0
        GLYPH %00011000,%00111000,%00011000,%00011000,%00011000,%00011000,%01111110,%00000000 ; 1
        GLYPH %00111100,%01100110,%00000110,%00001100,%00110000,%01100000,%01111110,%00000000 ; 2
        GLYPH %00111100,%01100110,%00000110,%00011100,%00000110,%01100110,%00111100,%00000000 ; 3
        GLYPH %00001100,%00011100,%00111100,%01101100,%01111110,%00001100,%00001100,%00000000 ; 4
        GLYPH %01111110,%01100000,%01111100,%00000110,%00000110,%01100110,%00111100,%00000000 ; 5
        GLYPH %00111100,%01100000,%01111100,%01100110,%01100110,%01100110,%00111100,%00000000 ; 6
        GLYPH %01111110,%00000110,%00001100,%00011000,%00110000,%00110000,%00110000,%00000000 ; 7
        GLYPH %00111100,%01100110,%00111100,%01100110,%01100110,%01100110,%00111100,%00000000 ; 8
        GLYPH %00111100,%01100110,%01100110,%00111110,%00000110,%00000110,%00111100,%00000000 ; 9

        ; --- 切替ゾーン（bank1 側, $FF00）---
        ORG  $1F00
        RORG $FF00
        ds 3, $EA
        jsr TitleScene      ; $FF03
        lda $FFF8           ; $FF06 → bank0

        ORG  $1FE0
        RORG $FFE0
        lda $FFF8
        jmp $F000

        ORG  $1FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0
