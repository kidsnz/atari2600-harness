#!/bin/bash
# stella_oracle.sh — Stella オラクル照合の全自動化（R5, v1.33.0）
# 使い方:  scripts/stella_oracle.sh <rom.bin> [frames]
# 仕組み:  cmd/stellacheck が Stella を起動し dump を待つ。本スクリプトは並行で
#          osascript により Stella へバッククォートキーを送出（デバッガ突入→autoexec 実行）。
# 必要条件（初回のみ・人間の1クリック）:
#   システム設定 → プライバシーとセキュリティ → アクセシビリティ → このターミナル(または iTerm 等)を許可
set -u
ROM="${1:?usage: stella_oracle.sh <rom.bin> [frames]}"
FRAMES="${2:-5}"
cd "$(dirname "$0")/.."

# --- アクセシビリティ許可のプリフライト ---
if ! osascript -e 'tell application "System Events" to get name of first process' >/dev/null 2>&1; then
  echo "✋ アクセシビリティ許可が必要です（初回のみ）:"
  echo "   システム設定 → プライバシーとセキュリティ → アクセシビリティ → このターミナルをON"
  echo "   許可後にもう一度このスクリプトを実行してください。"
  echo "   （許可するまでは従来どおり手動で \` キーを押す運用でも動きます: go run ./cmd/stellacheck -rom ...）"
  exit 2
fi

( sleep 4
  osascript \
    -e 'tell application "Stella" to activate' \
    -e 'delay 1' \
    -e 'tell application "System Events" to keystroke "`"' ) &
KEYPID=$!
go run ./cmd/stellacheck -rom "$ROM" -frames "$FRAMES"
RC=$?
kill "$KEYPID" 2>/dev/null
exit $RC
