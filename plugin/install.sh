#!/usr/bin/env bash
# Install pillow's Claude Code plugin (hooks).
# This copies the hook configuration into Claude Code's settings.
set -euo pipefail

CLAUDE_DIR="${HOME}/.claude"
SETTINGS_FILE="${CLAUDE_DIR}/settings.json"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOKS_DIR="${SCRIPT_DIR}/hooks"

echo "Installing pillow Claude Code plugin..."

# Ensure hooks are executable
chmod +x "${HOOKS_DIR}"/*.sh

# Check if Claude Code settings exist
if [ ! -f "$SETTINGS_FILE" ]; then
  mkdir -p "$CLAUDE_DIR"
  echo '{}' > "$SETTINGS_FILE"
fi

# Create a pillow-hook wrapper that routes to the right hook script
HOOK_BIN="/usr/local/bin/pillow-hook"
if command -v pillow > /dev/null 2>&1; then
  PILLOW_BIN=$(command -v pillow)
  PILLOW_DIR=$(dirname "$PILLOW_BIN")
fi

cat > /tmp/pillow-hook << 'HOOKEOF'
#!/usr/bin/env bash
# pillow-hook — routes Claude Code hook invocations to the right handler.
HOOK_TYPE="$1"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../share/pillow/hooks" 2>/dev/null || echo "${HOME}/.config/pillow/hooks")"

# Find hooks directory
for dir in \
  "${PILLOW_HOOKS_DIR:-}" \
  "${HOME}/.config/pillow/hooks" \
  "/usr/local/share/pillow/hooks" \
  "$(dirname "$0")/../share/pillow/hooks"; do
  if [ -d "$dir" ]; then
    SCRIPT_DIR="$dir"
    break
  fi
done

case "$HOOK_TYPE" in
  preToolUse)   exec "${SCRIPT_DIR}/preToolUse.sh" ;;
  postToolUse)  exec "${SCRIPT_DIR}/postToolUse.sh" ;;
  sessionStart) exec "${SCRIPT_DIR}/sessionStart.sh" ;;
  sessionEnd)   exec "${SCRIPT_DIR}/sessionEnd.sh" ;;
  *) echo "unknown hook: $HOOK_TYPE" >&2; exit 1 ;;
esac
HOOKEOF

# Install hooks to config dir
HOOKS_DEST="${HOME}/.config/pillow/hooks"
mkdir -p "$HOOKS_DEST"
cp "${HOOKS_DIR}"/*.sh "$HOOKS_DEST/"
chmod +x "$HOOKS_DEST"/*.sh

echo "  Hooks installed to ${HOOKS_DEST}"

# Install the hook router
if [ -w /usr/local/bin ] || [ -w "$(dirname "$(command -v pillow 2>/dev/null || echo /usr/local/bin/x)")" ]; then
  sudo cp /tmp/pillow-hook "$HOOK_BIN" 2>/dev/null || cp /tmp/pillow-hook "$HOOK_BIN"
  chmod +x "$HOOK_BIN"
  echo "  Hook router installed to ${HOOK_BIN}"
else
  LOCAL_BIN="${HOME}/.local/bin"
  mkdir -p "$LOCAL_BIN"
  cp /tmp/pillow-hook "${LOCAL_BIN}/pillow-hook"
  chmod +x "${LOCAL_BIN}/pillow-hook"
  echo "  Hook router installed to ${LOCAL_BIN}/pillow-hook"
  echo "  Make sure ${LOCAL_BIN} is in your PATH"
fi
rm -f /tmp/pillow-hook

# Merge hooks into Claude Code settings
echo "  Merging hooks into ${SETTINGS_FILE}..."
HOOKS_JSON=$(cat "${SCRIPT_DIR}/hooks.json")

# Use jq to merge if available
if command -v jq > /dev/null 2>&1; then
  MERGED=$(jq -s '.[0] * .[1]' "$SETTINGS_FILE" <(echo "$HOOKS_JSON"))
  echo "$MERGED" > "$SETTINGS_FILE"
  echo "  Claude Code settings updated."
else
  echo "  jq not found — please manually merge ${SCRIPT_DIR}/hooks.json into ${SETTINGS_FILE}"
fi

echo ""
echo "Done! Start the pillow daemon with 'pillow' and then use Claude Code normally."
