#!/usr/bin/env bash
# pillow SessionEnd hook — called by Claude Code when a session ends.
# Fire-and-forget: silently exits if the daemon is unreachable.

SOCKET="${PILLOW_SOCKET:-/tmp/pillow.sock}"

[ -S "$SOCKET" ] || exit 0
command -v jq > /dev/null 2>&1 || exit 0

PAYLOAD=$(cat) || exit 0

SESSION_ID=$(echo "$PAYLOAD" | jq -r '.session_id // ""' 2>/dev/null) || SESSION_ID=""

BODY=$(jq -n \
  --arg session_id "$SESSION_ID" \
  '{session_id: $session_id}' 2>/dev/null) || exit 0

curl -sf --max-time 2 --unix-socket "$SOCKET" \
  -X POST http://pillow/session/end \
  -H "Content-Type: application/json" \
  -d "$BODY" > /dev/null 2>&1 || true
