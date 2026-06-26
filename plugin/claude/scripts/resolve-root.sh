#!/bin/sh
# resolve-root.sh - resolve the stardust workspace mode and root.
#
# Prints two eval-safe lines for hook scripts and commands to consume:
#   MODE=<repo|vault|none>
#   ROOT='<path>'
#
# Resolution order (see ADR 0005):
#   1. ${CLAUDE_PROJECT_DIR}/.stardust is a directory -> (repo, project dir)
#   2. config.json has mode vault or auto and an existing vaultPath -> (vault, vaultPath)
#   3. otherwise -> (none, "")
#
# Always exits 0. Writes nothing to stderr on the normal paths.

set -u

emit() {
  # emit <mode> <root>. Single-quotes the root so eval keeps spaces intact.
  _mode=$1
  _root=$2
  _esc=$(printf '%s' "$_root" | sed "s/'/'\\\\''/g")
  printf 'MODE=%s\n' "$_mode"
  printf "ROOT='%s'\n" "$_esc"
}

project_dir=${CLAUDE_PROJECT_DIR:-}
plugin_data=${CLAUDE_PLUGIN_DATA:-}

# 1. Repo mode: an initialized stardust workspace at the project root.
if [ -n "$project_dir" ] && [ -d "$project_dir/.stardust" ]; then
  emit repo "$project_dir"
  exit 0
fi

# 2. Vault mode: a configured vault path that exists on disk.
config="$plugin_data/config.json"
if [ -n "$plugin_data" ] && [ -f "$config" ] && command -v jq >/dev/null 2>&1; then
  mode=$(jq -r '.mode // empty' "$config" 2>/dev/null)
  vault=$(jq -r '.vaultPath // empty' "$config" 2>/dev/null)
  if { [ "$mode" = "vault" ] || [ "$mode" = "auto" ]; } && [ -n "$vault" ] && [ -d "$vault" ]; then
    emit vault "$vault"
    exit 0
  fi
fi

# 3. No workspace. Drives graceful degradation.
emit none ""
exit 0
