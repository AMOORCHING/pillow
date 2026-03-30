#!/usr/bin/env bash
# pillow SessionStart hook — called by Claude Code when a session begins.
# Fire-and-forget: silently exits if the daemon is unreachable.

SOCKET="${PILLOW_SOCKET:-/tmp/pillow.sock}"

[ -S "$SOCKET" ] || exit 0
command -v jq > /dev/null 2>&1 || exit 0

PAYLOAD=$(cat) || exit 0

SESSION_ID=$(echo "$PAYLOAD" | jq -r '.session_id // ""' 2>/dev/null) || SESSION_ID=""
GOAL=$(echo "$PAYLOAD" | jq -r '.prompt // .goal // ""' 2>/dev/null) || GOAL=""

BODY=$(jq -n \
  --arg session_id "$SESSION_ID" \
  --arg goal "$GOAL" \
  '{session_id: $session_id, goal: $goal}' 2>/dev/null) || exit 0

curl -sf --max-time 2 --unix-socket "$SOCKET" \
  -X POST http://pillow/session/start \
  -H "Content-Type: application/json" \
  -d "$BODY" > /dev/null 2>&1 || true
