#!/usr/bin/env bash
# pillow PostToolUse hook — called by Claude Code after each tool invocation.
# Fire-and-forget: silently exits if the daemon is unreachable.

SOCKET="${PILLOW_SOCKET:-/tmp/pillow.sock}"

[ -S "$SOCKET" ] || exit 0
command -v jq > /dev/null 2>&1 || exit 0

PAYLOAD=$(cat) || exit 0

TOOL=$(echo "$PAYLOAD" | jq -r '.tool_name // .tool // ""' 2>/dev/null) || TOOL=""
INPUT=$(echo "$PAYLOAD" | jq -c '.tool_input // .input // {}' 2>/dev/null) || INPUT='{}'
OUTPUT=$(echo "$PAYLOAD" | jq -r '.tool_output // .output // ""' 2>/dev/null | head -c 500) || OUTPUT=""
SESSION_ID=$(echo "$PAYLOAD" | jq -r '.session_id // ""' 2>/dev/null) || SESSION_ID=""

EVENT=$(jq -n \
  --arg type "postToolUse" \
  --arg tool "$TOOL" \
  --arg session_id "$SESSION_ID" \
  --argjson input "$INPUT" \
  --arg output "$OUTPUT" \
  '{type: $type, tool: $tool, session_id: $session_id, input: $input, output: $output}' 2>/dev/null) || exit 0

curl -sf --max-time 2 --unix-socket "$SOCKET" \
  -X POST http://pillow/event \
  -H "Content-Type: application/json" \
  -d "$EVENT" > /dev/null 2>&1 || true
