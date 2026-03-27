#!/usr/bin/env bash
# pillow SessionEnd hook — called by Claude Code when a session ends.
set -euo pipefail

SOCKET="${PILLOW_SOCKET:-/tmp/pillow.sock}"

PAYLOAD=$(cat)

SESSION_ID=$(echo "$PAYLOAD" | jq -r '.session_id // ""')

BODY=$(jq -n \
  --arg session_id "$SESSION_ID" \
  '{session_id: $session_id}')

curl -s --unix-socket "$SOCKET" \
  -X POST http://pillow/session/end \
  -H "Content-Type: application/json" \
  -d "$BODY" > /dev/null 2>&1 || true
