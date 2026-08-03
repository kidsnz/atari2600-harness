#!/bin/bash
# stella_oracle.sh — Stella オラクル照合の全自動化（R5, v1.33.0 / tia モード v2.1.0）
# 使い方:  scripts/stella_oracle.sh <rom.bin> [frames] [pixels|tia]
#
# 仕組み（ram / pixels モード）:
#   cmd/stellacheck が Stella を起動し dump を待つ。本スクリプトは並行で osascript により
#   Stella へバッククォートキーを送出（デバッガ突入→autoexec 実行）。
#
# 仕組み（tia モード = G4 書込専用レジスタ照合）:
#   Stella は書込専用 TIA レジスタ（COLUPF/NUSIZ/HMxx…）を **デバッガの `tia` コマンド**でしか
#   出さない。しかも `tia` の出力は prompt widget にしか行かず、autoexec.script 経由では
#   取り出せない（Debugger::exec は各コマンドの出力を捨て "Executed N commands" しか残さない
#   ＝ autoexec 内の saveSes は 0 バイトになる。2026-08-03 実測）。`dump 00 3f 1` も届かない
#   （TIA の **読み出し**ポート＝衝突/INPT が $10 ごとにミラーされて返るだけ。実測）。
#   よってここでは Stella を起動 → ` でデバッガ突入 → プロンプトへ `tia` と `saveSes` を
#   **クリップボード貼り付け**で送る（keystroke だと日本語 IME に食われるため paste 経由）。
#   保存先は Stella の user dir（~/Desktop、-userdir では変わらない＝実測）なので、
#   取得後ただちに internal/oracle/testdata/stella_tia/<rom>.txt へ退避する。
#
# 必要条件（初回のみ・人間の1クリック）:
#   システム設定 → プライバシーとセキュリティ → アクセシビリティ → このターミナル(または iTerm 等)を許可
set -u
ROM="${1:?usage: stella_oracle.sh <rom.bin> [frames] [pixels|tia]}"
FRAMES="${2:-5}"
MODE="${3:-}"
cd "$(dirname "$0")/.."

# --- アクセシビリティ許可のプリフライト ---
if ! osascript -e 'tell application "System Events" to get name of first process' >/dev/null 2>&1; then
  echo "✋ アクセシビリティ許可が必要です（初回のみ）:"
  echo "   システム設定 → プライバシーとセキュリティ → アクセシビリティ → このターミナルをON"
  echo "   許可後にもう一度このスクリプトを実行してください。"
  echo "   （許可するまでは従来どおり手動で \` キーを押す運用でも動きます: go run ./cmd/stellacheck -rom ...）"
  exit 2
fi

STELLA=/Applications/Stella.app/Contents/MacOS/Stella
AUTOEXEC="$HOME/Library/Application Support/Stella/autoexec.script"
CAPDIR=internal/oracle/testdata/stella_tia

frontmost() {
  osascript -e 'tell application "System Events" to get name of first application process whose frontmost is true' 2>/dev/null
}
# Stella を最前面に戻す。別プロセス（ブラウザ等）に焦点を奪われても打鍵前に取り返す＝
# 打鍵が無関係なアプリに落ちる事故を構造的に防ぐ。
ensure_front() {
  for _ in 1 2 3 4 5 6; do
    osascript -e 'tell application "Stella" to activate' >/dev/null 2>&1
    /bin/sleep 0.5
    [ "$(frontmost)" = "Stella" ] && return 0
  done
  return 1
}

capture_tia() {
  local rom="$1" frames="$2" out="$3"
  local base clip
  base="$(basename "${rom%.*}")"
  clip="$(mktemp)"; pbpaste > "$clip" 2>/dev/null

  printf 'reset\nframe %s\n' "$frames" > "$AUTOEXEC"
  # 既存の session_*.txt は「我々が作っていないファイル」なので触らない。起動前の一覧を
  # 控えて、あとで **新規に現れた 1 本だけ** を自分の成果物として扱う。
  local before; before="$(mktemp)"
  ls "$HOME/Desktop"/session_*.txt 2>/dev/null > "$before"
  "$STELLA" -dbg.res 1000x700 -dbg.fontsize small "$rom" >/dev/null 2>&1 &
  local pid=$!
  /bin/sleep 3
  local rc=0
  if ensure_front; then
    osascript -e 'tell application "System Events" to keystroke "`"'
    /bin/sleep 2
    local cmd
    for cmd in tia saveSes; do
      printf '%s' "$cmd" | pbcopy
      ensure_front || { rc=4; break; }
      osascript -e 'tell application "System Events" to keystroke "v" using command down'
      /bin/sleep 0.4
      osascript -e 'tell application "System Events" to key code 36'
      /bin/sleep 0.6
    done
    /bin/sleep 0.8
  else
    rc=3
  fi
  kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
  pbcopy < "$clip"; rm -f "$clip"
  [ "$rc" -ne 0 ] && { echo "capture failed (rc=$rc: could not keep Stella frontmost)"; return "$rc"; }

  local ses
  ses="$(ls -t "$HOME/Desktop"/session_*.txt 2>/dev/null | grep -vxFf "$before" | head -1)"
  rm -f "$before"
  [ -z "$ses" ] && { echo "capture failed: Stella wrote no session file"; return 5; }
  mkdir -p "$(dirname "$out")"
  {
    echo "# stella-tia-oracle capture"
    echo "# rom: $rom"
    echo "# frames: $frames"
    echo "# stella: $($STELLA -help 2>&1 | head -1)"
    echo "# captured: $(date +%Y-%m-%d)"
    cat "$ses"
  } > "$out"
  rm -f "$ses"
  echo "captured: $out"
}

case "$MODE" in
  tia)
    OUT="$CAPDIR/$(basename "${ROM%.*}").txt"
    capture_tia "$ROM" "$FRAMES" "$OUT" || exit $?
    # コーパス一括取得では 1 本ごとに go run するとビルドの方が長い。取得だけして
    # 採点は `go test ./internal/oracle` にまとめる用の逃げ道。
    [ "${STELLA_CAPTURE_ONLY:-}" = "1" ] && exit 0
    go run ./cmd/stellacheck -session "$OUT"
    ;;
  pixels)
    ( sleep 4
      osascript -e 'tell application "Stella" to activate' -e 'delay 1' \
        -e 'tell application "System Events" to keystroke "`"' ) &
    KEYPID=$!
    go run ./cmd/stellacheck -rom "$ROM" -frames "$FRAMES" -pixels
    RC=$?
    kill "$KEYPID" 2>/dev/null
    exit $RC
    ;;
  *)
    ( sleep 4
      osascript -e 'tell application "Stella" to activate' -e 'delay 1' \
        -e 'tell application "System Events" to keystroke "`"' ) &
    KEYPID=$!
    go run ./cmd/stellacheck -rom "$ROM" -frames "$FRAMES"
    RC=$?
    kill "$KEYPID" 2>/dev/null
    exit $RC
    ;;
esac
