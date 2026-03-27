#!/usr/bin/env bash
# pillow SessionStart hook — called by Claude Code when a session begins.
set -euo pipefail

SOCKET="${PILLOW_SOCKET:-/tmp/pillow.sock}"

PAYLOAD=$(cat)

SESSION_ID=$(echo "$PAYLOAD" | jq -r '.session_id // ""')
GOAL=$(echo "$PAYLOAD" | jq -r '.prompt // .goal // ""')

BODY=$(jq -n \
  --arg session_id "$SESSION_ID" \
  --arg goal "$GOAL" \
  '{session_id: $session_id, goal: $goal}')

curl -s --unix-socket "$SOCKET" \
  -X POST http://pillow/session/start \
  -H "Content-Type: application/json" \
  -d "$BODY" > /dev/null 2>&1 || true
