#!/bin/sh
# prompt-submit.sh - optional, debounced retrieval reminder on UserPromptSubmit.
#
# Off unless midConversationReminders is true in config. When on, it emits at
# most one short <system-reminder> steering to stardust, and only when the
# prompt shows retrieval intent and no reminder fired within the debounce
# window. Otherwise it prints nothing. It never errors and never nags.

set -u

# The hook payload (JSON with a prompt field) arrives on stdin.
payload=$(cat 2>/dev/null)

plugin_data=${CLAUDE_PLUGIN_DATA:-}
config="$plugin_data/config.json"

# Need a state dir, a config, and jq to do anything at all.
[ -n "$plugin_data" ] || exit 0
[ -f "$config" ] || exit 0
command -v jq >/dev/null 2>&1 || exit 0

# Off unless explicitly enabled.
enabled=$(jq -r '.midConversationReminders // false' "$config" 2>/dev/null)
[ "$enabled" = "true" ] || exit 0

# Extract the prompt text.
prompt=$(printf '%s' "$payload" | jq -r '.prompt // empty' 2>/dev/null)
[ -n "$prompt" ] || exit 0

# Only steer on retrieval intent.
lc=$(printf '%s' "$prompt" | tr '[:upper:]' '[:lower:]')
case "$lc" in
  *context*|*"find "*|*"where is"*|*"what is the plan"*|*"latest plan"*|*ticket*|*spec*|*"prior decision"*|*"did we decide"*|*"get up to speed"*)
    : ;;
  *)
    exit 0 ;;
esac

# Debounce: skip if a reminder fired within the window.
window=900
marker="$plugin_data/.reminder-last"
now=$(date +%s)
if [ -f "$marker" ]; then
  last=$(cat "$marker" 2>/dev/null)
  case "$last" in
    '' | *[!0-9]*) last=0 ;;
  esac
  if [ "$last" -gt 0 ] && [ "$((now - last))" -lt "$window" ]; then
    exit 0
  fi
fi

mkdir -p "$plugin_data" 2>/dev/null || exit 0
printf '%s' "$now" > "$marker" 2>/dev/null || true

printf '<system-reminder>For this, route context through stardust: run stardust bundle "<task>" (or the MCP bundle tool) before grepping or opening files by hand.</system-reminder>\n'
exit 0
