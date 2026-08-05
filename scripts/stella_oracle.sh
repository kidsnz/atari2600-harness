#!/bin/bash
# stella_oracle.sh — fully automated Stella oracle cross-check (R5, v1.33.0 / tia mode v2.1.0)
# Usage:  scripts/stella_oracle.sh <rom.bin> [frames] [pixels|tia]
#
# How it works (ram / pixels mode):
#   cmd/stellacheck launches Stella and waits for the dump. This script concurrently sends the
#   backquote key to Stella via osascript (enter the debugger -> autoexec runs).
#
# How it works (tia mode = G4 write-only register cross-check):
#   Stella exposes the write-only TIA registers (COLUPF/NUSIZ/HMxx...) ONLY through the
#   **debugger's `tia` command**. Moreover `tia`'s output goes only to the prompt widget and
#   cannot be extracted via autoexec.script (Debugger::exec discards each command's output and
#   leaves only "Executed N commands" = a saveSes inside autoexec produces 0 bytes. Measured
#   2026-08-03). `dump 00 3f 1` does not reach them either (only the TIA's **read** ports =
#   collisions/INPT come back, mirrored every $10. Measured).
#   So here we launch Stella -> enter the debugger with ` -> send `tia` and `saveSes` to the
#   prompt via **clipboard paste** (keystroke gets eaten by the Japanese IME, hence paste).
#   The save location is Stella's user dir (~/Desktop; -userdir does not change it = measured),
#   so immediately after capture the file is moved to internal/oracle/testdata/stella_tia/<rom>.txt.
#
# Prerequisite (first run only; one human click):
#   System Settings -> Privacy & Security -> Accessibility -> allow this terminal (or iTerm etc.)
set -u
ROM="${1:?usage: stella_oracle.sh <rom.bin> [frames] [pixels|tia]}"
FRAMES="${2:-5}"
MODE="${3:-}"
cd "$(dirname "$0")/.."

# --- Accessibility-permission preflight ---
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
# Bring Stella back to the front. Even if another process (a browser etc.) steals focus, take it
# back before typing = structurally prevents the accident of keystrokes landing in an unrelated app.
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
  # Pre-existing session_*.txt are "files we did not create", so leave them alone. Note the
  # pre-launch listing, and later treat **only the single newly appeared file** as our artifact.
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
    # In a whole-corpus batch capture, a go run per ROM spends longer building than capturing.
    # Escape hatch for capturing only, and batching the grading into `go test ./internal/oracle`.
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
