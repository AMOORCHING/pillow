#!/usr/bin/env bash
# pillow PreToolUse hook — called by Claude Code before each tool invocation.
# Reads hook JSON from stdin, forwards to pillow daemon, returns decision.
set -euo pipefail

SOCKET="${PILLOW_SOCKET:-/tmp/pillow.sock}"

# Read hook payload from stdin
PAYLOAD=$(cat)

# Extract tool name and input from the hook payload
TOOL=$(echo "$PAYLOAD" | jq -r '.tool_name // .tool // ""')
INPUT=$(echo "$PAYLOAD" | jq -c '.tool_input // .input // {}')
SESSION_ID=$(echo "$PAYLOAD" | jq -r '.session_id // ""')

# Build event JSON
EVENT=$(jq -n \
  --arg type "preToolUse" \
  --arg tool "$TOOL" \
  --arg session_id "$SESSION_ID" \
  --argjson input "$INPUT" \
  '{type: $type, tool: $tool, session_id: $session_id, input: $input}')

# Send event to daemon
RESPONSE=$(curl -s --unix-socket "$SOCKET" \
  -X POST http://pillow/event \
  -H "Content-Type: application/json" \
  -d "$EVENT" 2>/dev/null || echo '{"classify":"none"}')

CLASSIFY=$(echo "$RESPONSE" | jq -r '.classify // "none"')
REASON=$(echo "$RESPONSE" | jq -r '.reason // ""')

# Also poll for slap
SLAP_RESPONSE=$(curl -s --unix-socket "$SOCKET" \
  -X GET http://pillow/slap 2>/dev/null || true)

SLAP_STATUS=$?
if [ -n "$SLAP_RESPONSE" ] && echo "$SLAP_RESPONSE" | jq -e '.timestamp' > /dev/null 2>&1; then
  # Slap detected — block with negotiation message
  echo '{"decision":"block","message":"🛑 pillow: Paused — slap detected.\nReply: \"stop\" / \"finish this file\" / \"revert\" / or type a redirect"}'
  exit 0
fi

# Return decision based on classification
case "$CLASSIFY" in
  "block")
    echo "{\"decision\":\"block\",\"message\":\"🛑 pillow: ${REASON}\"}"
    ;;
  *)
    echo '{"decision":"allow"}'
    ;;
esac
