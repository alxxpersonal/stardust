#!/bin/sh
# session-start.sh - SessionStart hook for the stardust-plugin.
#
# Emits, as additionalContext, the static stardust-first policy, a cache
# boundary marker, then a freshly generated, read-only <workspace-state> block
# (active plans, recent specs, verification, next step). Total emission is kept
# small (target under 5 KB). It never reindexes or regenerates at boot.
#
# Degrades to a silent no-op, or a single one-time pointer, when stardust is
# absent or no workspace resolves. It never errors loudly and never nags.

set -u

# --- Locate plugin files ---

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
plugin_root=${CLAUDE_PLUGIN_ROOT:-$(dirname -- "$script_dir")}
policy_file="$script_dir/policy.txt"
resolver="$plugin_root/scripts/resolve-root.sh"

# --- Helpers ---

# maybe_pointer <flag> <message>: print the message at most once ever, gated by
# a marker under CLAUDE_PLUGIN_DATA. Stays silent when there is no state dir, so
# a stock environment sees nothing.
maybe_pointer() {
  _flag=$1
  _msg=$2
  [ -n "${CLAUDE_PLUGIN_DATA:-}" ] || return 0
  mkdir -p "$CLAUDE_PLUGIN_DATA" 2>/dev/null || return 0
  _marker="$CLAUDE_PLUGIN_DATA/.$_flag"
  [ -e "$_marker" ] && return 0
  : > "$_marker" 2>/dev/null || return 0
  printf '%s\n' "$_msg"
}

# run_budgeted <cmd...>: run under a short wall-clock budget when a timeout tool
# is present, otherwise run directly. Keeps a slow check from stalling boot.
run_budgeted() {
  if command -v timeout >/dev/null 2>&1; then
    timeout 8 "$@"
  elif command -v gtimeout >/dev/null 2>&1; then
    gtimeout 8 "$@"
  else
    "$@"
  fi
}

# extract_section_rows <section> <index_file>: print the data rows of a pipe
# table under a "## <section>" header, skipping the header and separator rows.
extract_section_rows() {
  _sec="## $1"
  _idx=$2
  [ -f "$_idx" ] || return 0
  awk -v sec="$_sec" '
    $0 == sec { insec = 1; rn = 0; next }
    /^## / { insec = 0 }
    insec && /^\|/ {
      rn++
      if (rn <= 2) next
      print
    }
  ' "$_idx"
}

# format_rows <max>: format up to <max> pipe-table rows as "- Title [Status] Doc".
format_rows() {
  awk -F'|' -v max="$1" '
    {
      title = $2; status = $3; doc = $4
      gsub(/^[ \t]+|[ \t]+$/, "", title)
      gsub(/^[ \t]+|[ \t]+$/, "", status)
      gsub(/^[ \t]+|[ \t]+$/, "", doc)
      if (title == "") next
      n++
      if (n > max) next
      printf "  - %s [%s] %s\n", title, status, doc
    }'
}

# format_active <max>: like format_rows but drops settled statuses.
format_active() {
  awk -F'|' -v max="$1" '
    {
      title = $2; status = $3; doc = $4
      gsub(/^[ \t]+|[ \t]+$/, "", title)
      gsub(/^[ \t]+|[ \t]+$/, "", status)
      gsub(/^[ \t]+|[ \t]+$/, "", doc)
      if (title == "") next
      if (status == "Done" || status == "Implemented" || status == "Archived" || status == "Superseded") next
      n++
      if (n > max) next
      printf "  - %s [%s] %s\n", title, status, doc
    }'
}

# verification_line <root>: one line of check counts plus the indexed commit.
verification_line() {
  _root=$1
  _json=$(cd "$_root" 2>/dev/null && run_budgeted stardust check --output json 2>/dev/null)
  if [ -n "$_json" ] && printf '%s' "$_json" | jq -e . >/dev/null 2>&1; then
    _errs=$(printf '%s' "$_json" | jq '[(.issues // [])[] | select(.severity == "error")] | length' 2>/dev/null)
    _warns=$(printf '%s' "$_json" | jq '[(.issues // [])[] | select(.severity == "warn" or .severity == "warning")] | length' 2>/dev/null)
    _line="stardust check: ${_errs} errors, ${_warns} warnings."
  else
    _line="stardust check: not run this boot (run stardust check)."
  fi
  _sha=$(git -C "$_root" rev-parse --short HEAD 2>/dev/null)
  if [ -n "$_sha" ]; then
    _line="$_line Index at HEAD ${_sha}."
  fi
  printf '%s' "$_line"
}

# emit_state <mode> <root>: print the <workspace-state> block.
emit_state() {
  _mode=$1
  _root=$2
  _idx="$_root/docs/INDEX.md"
  _generated=$(date "+%Y-%m-%dT%H:%M:%S%z")

  printf '<workspace-state mode="%s" root="%s" generated="%s">\n' "$_mode" "$_root" "$_generated"

  printf '  <active-plans>\n'
  _plans=$(extract_section_rows "Plans" "$_idx" | format_active 5)
  if [ -n "$_plans" ]; then
    printf '%s\n' "$_plans"
  else
    printf '  - none active. Registry: docs/INDEX.md\n'
  fi
  printf '  </active-plans>\n'

  printf '  <recent-specs>\n'
  _specs=$(extract_section_rows "Specs" "$_idx" | format_rows 5)
  if [ -n "$_specs" ]; then
    printf '%s\n' "$_specs"
  else
    printf '  - none indexed. Registry: docs/INDEX.md\n'
  fi
  printf '  </recent-specs>\n'

  printf '  <verification>\n'
  printf '  %s\n' "$(verification_line "$_root")"
  printf '  </verification>\n'

  printf '  <next-step>\n'
  printf '  For any task, start with stardust bundle "<task>". Full registry: docs/INDEX.md\n'
  printf '  </next-step>\n'

  printf '</workspace-state>\n'
}

# --- Main flow ---

# Degrade quietly when stardust is unavailable.
if ! command -v stardust >/dev/null 2>&1; then
  maybe_pointer install-pointer-shown "<system-reminder>stardust-plugin is idle: the stardust binary is not on PATH. Install it with: go install github.com/alxxpersonal/stardust/cmd/stardust@latest</system-reminder>"
  exit 0
fi

# Resolve mode and root.
MODE=none
ROOT=""
if [ -f "$resolver" ]; then
  eval "$(sh "$resolver" 2>/dev/null)"
fi

# No workspace: one-time pointer, then silence.
if [ "$MODE" = "none" ] || [ -z "$ROOT" ]; then
  maybe_pointer setup-pointer-shown "<system-reminder>stardust-plugin: no workspace resolved. In a docs-convention repo run stardust init --docs, or configure a vault with /stardust:setup.</system-reminder>"
  exit 0
fi

# Resolved but uninitialized (for example a vault path without .stardust).
if [ ! -d "$ROOT/.stardust" ]; then
  maybe_pointer setup-pointer-shown "<system-reminder>stardust-plugin: $ROOT is not initialized. Run /stardust:setup or stardust init --docs there to enable workspace context.</system-reminder>"
  exit 0
fi

# Emit the static policy, the cache boundary, then the live state.
if [ -f "$policy_file" ]; then
  cat "$policy_file"
  printf '\n'
fi
printf '<!-- cache boundary: policy above is static and cacheable; state below regenerates per session -->\n\n'
emit_state "$MODE" "$ROOT"
exit 0
