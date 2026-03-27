#!/usr/bin/env bash
# pillow PostToolUse hook — called by Claude Code after each tool invocation.
set -euo pipefail

SOCKET="${PILLOW_SOCKET:-/tmp/pillow.sock}"

# Read hook payload from stdin
PAYLOAD=$(cat)

TOOL=$(echo "$PAYLOAD" | jq -r '.tool_name // .tool // ""')
INPUT=$(echo "$PAYLOAD" | jq -c '.tool_input // .input // {}')
OUTPUT=$(echo "$PAYLOAD" | jq -r '.tool_output // .output // ""' | head -c 500)
SESSION_ID=$(echo "$PAYLOAD" | jq -r '.session_id // ""')

EVENT=$(jq -n \
  --arg type "postToolUse" \
  --arg tool "$TOOL" \
  --arg session_id "$SESSION_ID" \
  --argjson input "$INPUT" \
  --arg output "$OUTPUT" \
  '{type: $type, tool: $tool, session_id: $session_id, input: $input, output: $output}')

# Send event to daemon (fire and forget — don't block the agent)
curl -s --unix-socket "$SOCKET" \
  -X POST http://pillow/event \
  -H "Content-Type: application/json" \
  -d "$EVENT" > /dev/null 2>&1 || true
