# シナリオ回帰（P2 / 欠落D）— 形式リファレンス

「入力タイムライン＋数値アサーション」を 1 つの JSON で宣言し、ROM に対して自動 pass/fail する回帰の仕組み
（`internal/scenario` ＋ `cmd/scenario`、v0.18.0）。MCP 不要で CI に乗る。判定は全て数値（鉄則1）。

```
go run ./cmd/scenario roms/<game>/scenarios/*.json   # 全 pass で exit 0 / 失敗で exit 1
```

シナリオは `roms/<game>/scenarios/*.json` に置く（ROM パスはリポジトリルート相対）。

## スキーマ

```jsonc
{
  "rom": "roms/frogger/frogger.bin",   // 必須
  "tv_spec": "NTSC",                    // 省略時 NTSC
  "warmup_frames": 2,                   // 計測前の空走（起動安定。省略時 2）

  "inputs": [                           // D-2: 入力タイムライン（frame は warmup 後 0 起点）
    {"frame": 1, "player": 0, "action": "up", "pressed": true}
    // action: left|right|up|down|fire|center / そのフレームを走らせる前に適用
  ],

  "asserts": [                          // D-1: フレーム終了時点の瞬時数値条件
    {"at_frame": 0, "field": "ram.0x81", "op": "==", "value": 144},
    {"at_frame": 1, "field": "ram.0x81", "op": "==", "value": 128}
  ],

  "checks": {                           // run 全体の性質（副作用のある計測。タイムライン後に評価）
    "ntsc_frame_lines": 262,            // StepFrame() == 262
    "max_line_budget": 76,              // 予算ガードが超過しない（assert_line_budget 相当）
    "golden_frame": true               // D-3: 描画フレーム連鎖ハッシュを <scenario>.golden と照合
  }
}
```

- 演算子 `op`: `==` `!=` `<` `<=` `>` `>=`。
- `value` は整数（bool フィールドは 0/1）。
- `at_frame` / `inputs.frame` は **warmup 後 0 起点**のフレーム番号。frame f は「入力適用 → 1 フレーム実行 → アサート評価」。

## フィールド語彙（`field`）

アサーションの語彙は `internal/emu` の read 系メソッドに 1 対 1 対応（観測ツールをそのまま回帰に使う）。
**未知フィールドはエラー**（タイポを握り潰さない）。

| field | 由来 |
|---|---|
| `frame` / `scanline` / `clock` | `Coords()` |
| `cycles_total` | `TotalCycles()` |
| `cpu.pc\|a\|x\|y\|sp` | `VCS.CPU` |
| `ram.0xNN`（16進/10進） | `PeekRAM` |
| `tia.<obj>.reset_pixel\|hmoved_pixel`（obj=player0/1,missile0/1,ball） | `read_tia` 相当 |
| `tiareg.player0\|player1.color\|nusiz\|reflected\|vertical_delay` | `ReadTIARegisters` |
| `tiareg.playfield.pf0\|pf1\|pf2\|foreground_color\|background_color\|ctrlpf\|reflected` | 〃 |
| `tiareg.ball.color\|enabled` | 〃 |
| `collisions.<pair>`（p0_p1, m0_p0, p0_pf, bl_pf …） | `ReadCollisions` |
| `audio.ch0\|ch1.control\|freq\|volume` | `ReadAudio` |

`checks`（run 全体）: `ntsc_frame_lines`（`StepFrame`）/ `max_line_budget`（`RunUntilBudget`）/
`golden_frame`（描画連鎖ハッシュ＝下記）。

## ゴールデンフレーム回帰（D-3, v0.19.0）

`checks.golden_frame: true` で、warmup を除いたタイムラインの**描画フレーム連鎖ハッシュ**（Gopher2600
`digest.Video` の sha1 連鎖）を `<scenario>.golden`（隣のファイル）と照合する＝描画ピクセルの回帰検知。

```
go run ./cmd/scenario -update roms/<game>/scenarios/foo.json   # 基準 .golden を記録/更新
go run ./cmd/scenario         roms/<game>/scenarios/foo.json   # 基準と照合（不一致で fail）
```

`.golden` が無い／`-update` 指定時は現在のハッシュを記録。ハッシュは warmup を除外して決定的
（同一 ROM＋同一入力＋同一フレーム数で再現）。ロジック/タイミング回帰（D-1/D-2）とは別レイヤの、
**描画そのものの回帰**を守る。`.golden` は git 追跡（基準として commit する）。

## 同梱サンプル

- `roms/litmus/scenarios/smoke.json` — `ram.0x80==$42` ＋ 262 行 ＋ 予算超過なし。
- `roms/litmus/scenarios/collide.json` — `collisions.bl_pf==1`（ball×全点灯 PF）。
- `roms/frogger/scenarios/boot.json` — FrogY 初期 144 ＋ 残機 3 ＋ 262 行 ＋ 予算超過なし。
- `roms/frogger/scenarios/hop.json` — `up` 入力で FrogY 144→128（入力タイムラインが実ゲームで効く実証）。
- `roms/frogger/scenarios/golden.json`（＋ `golden.golden`）— 描画フレーム連鎖ハッシュの回帰。

## スコープ外（次段）
- MCP ツール版 `run_scenario`（CLI とロジック共有で追加可能）。
- 範囲演算子・scanline 指定アサート・音声ゴールデン（`digest.Audio`）。
