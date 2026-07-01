#!/bin/sh
# resolve-root.sh - resolve the stardust workspace mode and root.
#
# Prints three eval-safe lines for hook scripts and commands to consume:
#   MODE=<repo|vault|none>
#   ROOT='<path>'
#   SOURCE=<env|cwd|project|vault-map|legacy|none>
#
# The MODE and ROOT lines are the stable contract; SOURCE is a diagnosis line so
# a misresolution is visible at a glance. Consumers reading MODE and ROOT are
# unaffected, and eval consumers simply gain a harmless variable.
#
# Resolution is layered, most local truth first, first hit wins (ADR 0038):
#   1. STARDUST_VAULT set to an existing dir -> repo if it holds .stardust/, else
#      vault. The explicit override always wins.                       (env)
#   2. Walk up from $PWD to /, first dir with .stardust/ -> repo.       (cwd)
#   3. Walk up from $CLAUDE_PROJECT_DIR the same way, when set -> repo. (project)
#   4. config.json mode vault|auto, .vaults map: exact $CLAUDE_PROJECT_DIR, exact
#      $PWD, then the longest key that is a path-prefix of either -> vault.
#                                                                       (vault-map)
#   5. Legacy top-level .vaultPath, only when no .vaults key exists,
#      mode vault|auto -> vault.                                        (legacy)
#   6. Otherwise -> none.                                               (none)
#
# Always exits 0. Writes nothing to stderr on the normal paths.

set -u

# --- Output ---

emit() {
  # emit <mode> <root> <source>. Single-quotes the root so eval keeps spaces,
  # commas, and apostrophes intact.
  _mode=$1
  _root=$2
  _source=$3
  _esc=$(printf '%s' "$_root" | sed "s/'/'\\\\''/g")
  printf 'MODE=%s\n' "$_mode"
  printf "ROOT='%s'\n" "$_esc"
  printf 'SOURCE=%s\n' "$_source"
}

# --- Walk-up ---

# walk_up <start>: print the first ancestor of <start> (inclusive) that contains
# a .stardust directory, resolved to a physical absolute path so symlinks, ..,
# and doubled slashes collapse. Prints nothing and returns 1 when none qualifies.
walk_up() {
  _start=$1
  [ -n "$_start" ] || return 1
  [ -d "$_start" ] || return 1
  _dir=$(CDPATH='' cd -P -- "$_start" 2>/dev/null && pwd -P) || return 1
  [ -n "$_dir" ] || return 1
  while : ; do
    if [ -d "$_dir/.stardust" ]; then
      printf '%s\n' "$_dir"
      return 0
    fi
    [ "$_dir" = "/" ] && return 1
    _parent=${_dir%/*}
    [ -n "$_parent" ] || _parent=/
    _dir=$_parent
  done
}

# --- Inputs ---

project_dir=${CLAUDE_PROJECT_DIR:-}
plugin_data=${CLAUDE_PLUGIN_DATA:-}
pwd_dir=${PWD:-$(pwd)}
override=${STARDUST_VAULT:-}

# --- Layer 1: STARDUST_VAULT override ---

if [ -n "$override" ] && [ -d "$override" ]; then
  root=$(CDPATH='' cd -P -- "$override" 2>/dev/null && pwd -P)
  [ -n "$root" ] || root=$override
  if [ -d "$root/.stardust" ]; then
    emit repo "$root" env
  else
    emit vault "$root" env
  fi
  exit 0
fi

# --- Layer 2: walk up from the current directory ---

if hit=$(walk_up "$pwd_dir"); then
  emit repo "$hit" cwd
  exit 0
fi

# --- Layer 3: walk up from the session start dir ---

if [ -n "$project_dir" ] && hit=$(walk_up "$project_dir"); then
  emit repo "$hit" project
  exit 0
fi

# --- Layers 4 and 5: config-driven vault modes ---

config="$plugin_data/config.json"
if [ -n "$plugin_data" ] && [ -f "$config" ] && command -v jq >/dev/null 2>&1; then
  mode=$(jq -r '.mode // empty' "$config" 2>/dev/null)
  if [ "$mode" = "vault" ] || [ "$mode" = "auto" ]; then

    # Layer 4: the per-project vaults map. Exact CLAUDE_PROJECT_DIR, exact PWD,
    # then the longest key that is a path-boundary prefix of either. Path
    # boundaries only: /a/b prefixes /a/b/c but not /a/bc.
    vault=$(jq -r --arg proj "$project_dir" --arg pwd "$pwd_dir" '
      def isp($k; $p):
        ($p != "")
        and (($p == $k)
             or ($p | startswith(if ($k | endswith("/")) then $k else $k + "/" end)));
      (.vaults) as $v
      | if ($v | type) != "object" then empty
        else
          (if ($proj != "") then $v[$proj] else null end) as $ep
          | (if ($pwd != "") then $v[$pwd] else null end) as $ew
          | if ($ep | type) == "string" then $ep
            elif ($ew | type) == "string" then $ew
            else
              ( [ $v | keys[]
                  | select((isp(.; $proj) or isp(.; $pwd))
                           and (($v[.] | type) == "string")) ]
                | sort_by(length) | last ) as $best
              | if $best == null then empty else $v[$best] end
            end
        end
    ' "$config" 2>/dev/null)
    if [ -n "$vault" ] && [ -d "$vault" ]; then
      emit vault "$vault" vault-map
      exit 0
    fi

    # Layer 5: the legacy top-level vaultPath, only when no vaults map exists.
    if [ "$(jq -r 'has("vaults")' "$config" 2>/dev/null)" != "true" ]; then
      vault=$(jq -r '.vaultPath // empty' "$config" 2>/dev/null)
      if [ -n "$vault" ] && [ -d "$vault" ]; then
        emit vault "$vault" legacy
        exit 0
      fi
    fi
  fi
fi

# --- Layer 6: no workspace. Drives graceful degradation. ---

emit none "" none
exit 0
