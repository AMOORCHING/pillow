#!/usr/bin/env bash
# pillow PreToolUse hook — called by Claude Code before each tool invocation.
# Reads hook JSON from stdin, forwards to pillow daemon, returns decision.
# Fails open (allows the tool call) if the daemon is unreachable or anything errors.

SOCKET="${PILLOW_SOCKET:-/tmp/pillow.sock}"

allow() { echo '{"decision":"allow"}'; exit 0; }

# Bail early if the daemon socket doesn't exist
[ -S "$SOCKET" ] || allow

# Bail if jq isn't available
command -v jq > /dev/null 2>&1 || allow

PAYLOAD=$(cat) || allow

TOOL=$(echo "$PAYLOAD" | jq -r '.tool_name // .tool // ""' 2>/dev/null) || TOOL=""
INPUT=$(echo "$PAYLOAD" | jq -c '.tool_input // .input // {}' 2>/dev/null) || INPUT='{}'
SESSION_ID=$(echo "$PAYLOAD" | jq -r '.session_id // ""' 2>/dev/null) || SESSION_ID=""

EVENT=$(jq -n \
  --arg type "preToolUse" \
  --arg tool "$TOOL" \
  --arg session_id "$SESSION_ID" \
  --argjson input "$INPUT" \
  '{type: $type, tool: $tool, session_id: $session_id, input: $input}' 2>/dev/null) || allow

RESPONSE=$(curl -sf --max-time 2 --unix-socket "$SOCKET" \
  -X POST http://pillow/event \
  -H "Content-Type: application/json" \
  -d "$EVENT" 2>/dev/null) || allow

CLASSIFY=$(echo "$RESPONSE" | jq -r '.classify // "none"' 2>/dev/null) || CLASSIFY="none"
REASON=$(echo "$RESPONSE" | jq -r '.reason // ""' 2>/dev/null) || REASON=""

SLAP_RESPONSE=$(curl -sf --max-time 1 --unix-socket "$SOCKET" \
  -X GET http://pillow/slap 2>/dev/null) || SLAP_RESPONSE=""

if [ -n "$SLAP_RESPONSE" ] && echo "$SLAP_RESPONSE" | jq -e '.timestamp' > /dev/null 2>&1; then
  echo '{"decision":"block","message":"🛑 pillow: Paused — slap detected.\nReply: \"stop\" / \"finish this file\" / \"revert\" / or type a redirect"}'
  exit 0
fi

case "$CLASSIFY" in
  "block")
    echo "{\"decision\":\"block\",\"message\":\"🛑 pillow: ${REASON}\"}"
    ;;
  *)
    allow
    ;;
esac
