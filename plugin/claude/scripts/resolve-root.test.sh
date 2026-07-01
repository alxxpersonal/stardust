#!/bin/sh
# resolve-root.test.sh - pin the six-layer workspace resolver.
#
# Self-contained: builds isolated temp dirs, drives resolve-root.sh with a
# controlled cwd and environment, and asserts the emitted MODE, ROOT, and
# SOURCE lines for each of the spec's ten verification cases.
#
# Exits nonzero on any failure so it can gate a commit.

set -u

# --- Locate the resolver (sibling of this test) ---

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd -P)
RESOLVER="$TEST_DIR/resolve-root.sh"

if [ ! -f "$RESOLVER" ]; then
  printf 'resolve-root.test.sh: resolver not found at %s\n' "$RESOLVER" >&2
  exit 2
fi

# --- Temp workspace ---

TMPROOT=$(mktemp -d "${TMPDIR:-/tmp}/resolve-root-test.XXXXXX") || {
  printf 'resolve-root.test.sh: mktemp failed\n' >&2
  exit 2
}
cleanup() { rm -rf "$TMPROOT"; }
trap cleanup EXIT INT TERM

FAILURES=0

# --- Helpers ---

# phys <dir>: print the physical absolute path of an existing dir, matching the
# resolver's own cd -P/pwd -P resolution (temp dirs live under symlinked /var).
phys() { ( CDPATH='' cd -P -- "$1" && pwd -P ); }

# run_case <cwd>: run the resolver with the process cwd at <cwd>. The caller
# sets or unsets STARDUST_VAULT/CLAUDE_PROJECT_DIR/CLAUDE_PLUGIN_DATA in the
# enclosing subshell first. Prints the resolver's three output lines.
run_case() {
  cd "$1" 2>/dev/null || { printf 'cd-failed\n'; return 1; }
  sh "$RESOLVER"
}

# assert_case <label> <resolver-output> <want-mode> <want-root> <want-source>
assert_case() {
  _label=$1
  _out=$2
  _wmode=$3
  _wroot=$4
  _wsource=$5

  MODE=none
  ROOT=""
  SOURCE=none
  eval "$_out" 2>/dev/null

  if [ "$MODE" = "$_wmode" ] && [ "$ROOT" = "$_wroot" ] && [ "$SOURCE" = "$_wsource" ]; then
    printf 'ok   - %s\n' "$_label"
  else
    printf 'FAIL - %s\n' "$_label"
    printf '        want MODE=%s ROOT=[%s] SOURCE=%s\n' "$_wmode" "$_wroot" "$_wsource"
    printf '        got  MODE=%s ROOT=[%s] SOURCE=%s\n' "$MODE" "$ROOT" "$SOURCE"
    FAILURES=$((FAILURES + 1))
  fi
}

# --- Case 1: cwd inside a .stardust dir, CLAUDE_PROJECT_DIR unset -> repo/cwd ---

case_1() {
  d="$TMPROOT/c1"
  mkdir -p "$d/.stardust"
  want=$(phys "$d")
  out=$(
    unset STARDUST_VAULT CLAUDE_PROJECT_DIR CLAUDE_PLUGIN_DATA
    run_case "$d"
  )
  assert_case "1. cwd inside .stardust dir, no project dir -> repo/cwd" \
    "$out" repo "$want" cwd
}

# --- Case 2: cwd in a nested subdirectory of a .stardust dir -> repo via walk-up ---

case_2() {
  d="$TMPROOT/c2"
  mkdir -p "$d/.stardust" "$d/nested/deep"
  want=$(phys "$d")
  out=$(
    unset STARDUST_VAULT CLAUDE_PROJECT_DIR CLAUDE_PLUGIN_DATA
    run_case "$d/nested/deep"
  )
  assert_case "2. cwd in nested subdir -> repo via walk-up/cwd" \
    "$out" repo "$want" cwd
}

# --- Case 3: cwd elsewhere, CLAUDE_PROJECT_DIR set to a workspace -> repo/project ---

case_3() {
  ws="$TMPROOT/c3ws"
  mkdir -p "$ws/.stardust"
  neutral="$TMPROOT/c3neutral"
  mkdir -p "$neutral"
  want=$(phys "$ws")
  out=$(
    unset STARDUST_VAULT CLAUDE_PLUGIN_DATA
    CLAUDE_PROJECT_DIR="$ws"
    export CLAUDE_PROJECT_DIR
    run_case "$neutral"
  )
  assert_case "3. cwd elsewhere, project dir set -> repo/project" \
    "$out" repo "$want" project
}

# --- Case 4: STARDUST_VAULT wins over both walks -> env ---

case_4() {
  cwdrepo="$TMPROOT/c4cwd"
  mkdir -p "$cwdrepo/.stardust"
  projrepo="$TMPROOT/c4proj"
  mkdir -p "$projrepo/.stardust"
  override="$TMPROOT/c4override"
  mkdir -p "$override"
  want=$(phys "$override")
  out=$(
    unset CLAUDE_PLUGIN_DATA
    STARDUST_VAULT="$override"
    CLAUDE_PROJECT_DIR="$projrepo"
    export STARDUST_VAULT CLAUDE_PROJECT_DIR
    run_case "$cwdrepo"
  )
  assert_case "4. STARDUST_VAULT overrides both walks -> vault/env" \
    "$out" vault "$want" env
}

# --- Case 5: vault map exact key hit for the project dir -> vault/vault-map ---

case_5() {
  proj="$TMPROOT/c5proj"
  mkdir -p "$proj"
  vault="$TMPROOT/c5vault"
  mkdir -p "$vault"
  neutral="$TMPROOT/c5neutral"
  mkdir -p "$neutral"
  data="$TMPROOT/c5data"
  mkdir -p "$data"
  cat > "$data/config.json" <<EOF
{ "mode": "auto", "vaults": { "$proj": "$vault" } }
EOF
  out=$(
    unset STARDUST_VAULT
    CLAUDE_PROJECT_DIR="$proj"
    CLAUDE_PLUGIN_DATA="$data"
    export CLAUDE_PROJECT_DIR CLAUDE_PLUGIN_DATA
    run_case "$neutral"
  )
  assert_case "5. vaults exact project-dir key -> vault/vault-map" \
    "$out" vault "$vault" vault-map
}

# --- Case 6: vault map longest boundary-prefix from a subdirectory -> vault ---

case_6() {
  proj="$TMPROOT/c6proj"
  mkdir -p "$proj/a/b/deep"
  vshort="$TMPROOT/c6vshort"
  mkdir -p "$vshort"
  vlong="$TMPROOT/c6vlong"
  mkdir -p "$vlong"
  vwrong="$TMPROOT/c6vwrong"
  mkdir -p "$vwrong"
  neutral="$TMPROOT/c6neutral"
  mkdir -p "$neutral"
  data="$TMPROOT/c6data"
  mkdir -p "$data"
  # Keys: proj (prefix), proj/a (longer prefix, must win), and c6pro (a string
  # prefix but NOT a path-boundary prefix, must be ignored).
  cat > "$data/config.json" <<EOF
{ "mode": "auto", "vaults": {
  "$proj": "$vshort",
  "$proj/a": "$vlong",
  "$TMPROOT/c6pro": "$vwrong"
} }
EOF
  out=$(
    unset STARDUST_VAULT
    CLAUDE_PROJECT_DIR="$proj/a/b/deep"
    CLAUDE_PLUGIN_DATA="$data"
    export CLAUDE_PROJECT_DIR CLAUDE_PLUGIN_DATA
    run_case "$neutral"
  )
  assert_case "6. vaults longest boundary-prefix from subdir -> vault/vault-map" \
    "$out" vault "$vlong" vault-map
}

# --- Case 7: legacy vaultPath, only when no vaults key -> vault/legacy ---

case_7() {
  vault="$TMPROOT/c7vault"
  mkdir -p "$vault"
  neutral="$TMPROOT/c7neutral"
  mkdir -p "$neutral"
  data="$TMPROOT/c7data"
  mkdir -p "$data"
  cat > "$data/config.json" <<EOF
{ "mode": "auto", "vaultPath": "$vault" }
EOF
  out=$(
    unset STARDUST_VAULT CLAUDE_PROJECT_DIR
    CLAUDE_PLUGIN_DATA="$data"
    export CLAUDE_PLUGIN_DATA
    run_case "$neutral"
  )
  assert_case "7a. legacy vaultPath, no vaults key -> vault/legacy" \
    "$out" vault "$vault" legacy

  # A vaults key (even empty) suppresses the legacy vaultPath fallback.
  data2="$TMPROOT/c7data2"
  mkdir -p "$data2"
  cat > "$data2/config.json" <<EOF
{ "mode": "auto", "vaults": {}, "vaultPath": "$vault" }
EOF
  out2=$(
    unset STARDUST_VAULT CLAUDE_PROJECT_DIR
    CLAUDE_PLUGIN_DATA="$data2"
    export CLAUDE_PLUGIN_DATA
    run_case "$neutral"
  )
  assert_case "7b. vaults key present suppresses legacy vaultPath -> none" \
    "$out2" none "" none
}

# --- Case 8: unmapped dir, nothing to walk to -> none (ADR 0037 isolation) ---

case_8() {
  other="$TMPROOT/c8other"
  mkdir -p "$other"
  othervault="$TMPROOT/c8othervault"
  mkdir -p "$othervault"
  myproj="$TMPROOT/c8myproj"
  mkdir -p "$myproj"
  neutral="$TMPROOT/c8neutral"
  mkdir -p "$neutral"
  data="$TMPROOT/c8data"
  mkdir -p "$data"
  cat > "$data/config.json" <<EOF
{ "mode": "auto", "vaults": { "$other": "$othervault" } }
EOF
  out=$(
    unset STARDUST_VAULT
    CLAUDE_PROJECT_DIR="$myproj"
    CLAUDE_PLUGIN_DATA="$data"
    export CLAUDE_PROJECT_DIR CLAUDE_PLUGIN_DATA
    run_case "$neutral"
  )
  assert_case "8. unmapped dir, another project mapped -> none (isolation)" \
    "$out" none "" none
}

# --- Case 9: precedence, cwd walk > project walk > vault map ---

case_9() {
  # 9a: cwd walk (layer 2) beats project walk (layer 3) and the map (layer 4).
  repoA="$TMPROOT/c9a_cwd"
  mkdir -p "$repoA/.stardust"
  repoB="$TMPROOT/c9a_proj"
  mkdir -p "$repoB/.stardust"
  vault="$TMPROOT/c9a_vault"
  mkdir -p "$vault"
  data="$TMPROOT/c9a_data"
  mkdir -p "$data"
  cat > "$data/config.json" <<EOF
{ "mode": "auto", "vaults": { "$repoA": "$vault", "$repoB": "$vault" } }
EOF
  wantA=$(phys "$repoA")
  out=$(
    unset STARDUST_VAULT
    CLAUDE_PROJECT_DIR="$repoB"
    CLAUDE_PLUGIN_DATA="$data"
    export CLAUDE_PROJECT_DIR CLAUDE_PLUGIN_DATA
    run_case "$repoA"
  )
  assert_case "9a. cwd walk beats project walk and map -> repo/cwd" \
    "$out" repo "$wantA" cwd

  # 9b: project walk (layer 3) beats the vault map (layer 4).
  repoP="$TMPROOT/c9b_proj"
  mkdir -p "$repoP/.stardust"
  vaultB="$TMPROOT/c9b_vault"
  mkdir -p "$vaultB"
  neutral="$TMPROOT/c9b_neutral"
  mkdir -p "$neutral"
  data2="$TMPROOT/c9b_data"
  mkdir -p "$data2"
  cat > "$data2/config.json" <<EOF
{ "mode": "auto", "vaults": { "$repoP": "$vaultB" } }
EOF
  wantP=$(phys "$repoP")
  out2=$(
    unset STARDUST_VAULT
    CLAUDE_PROJECT_DIR="$repoP"
    CLAUDE_PLUGIN_DATA="$data2"
    export CLAUDE_PROJECT_DIR CLAUDE_PLUGIN_DATA
    run_case "$neutral"
  )
  assert_case "9b. project walk beats vault map -> repo/project" \
    "$out2" repo "$wantP" project
}

# --- Case 10: paths with spaces, commas, and an apostrophe resolve and quote ---

case_10() {
  base="$TMPROOT/c10 alx's space, comma"
  repo="$base/repo dir"
  mkdir -p "$repo/.stardust"
  wantrepo=$(phys "$repo")
  out=$(
    unset STARDUST_VAULT CLAUDE_PROJECT_DIR CLAUDE_PLUGIN_DATA
    run_case "$repo"
  )
  assert_case "10a. spaced/apostrophe path walk-up -> repo/cwd" \
    "$out" repo "$wantrepo" cwd

  # Vault-map path with spaces flows through JSON and eval-safe quoting.
  proj="$base/proj space"
  mkdir -p "$proj"
  vault="$base/vault space"
  mkdir -p "$vault"
  neutral="$base/neutral space"
  mkdir -p "$neutral"
  data="$base/data space"
  mkdir -p "$data"
  cat > "$data/config.json" <<EOF
{ "mode": "auto", "vaults": { "$proj": "$vault" } }
EOF
  out2=$(
    unset STARDUST_VAULT
    CLAUDE_PROJECT_DIR="$proj"
    CLAUDE_PLUGIN_DATA="$data"
    export CLAUDE_PROJECT_DIR CLAUDE_PLUGIN_DATA
    run_case "$neutral"
  )
  assert_case "10b. spaced vault-map path -> vault/vault-map" \
    "$out2" vault "$vault" vault-map
}

# --- Run all ---

case_1
case_2
case_3
case_4
case_5
case_6
case_7
case_8
case_9
case_10

printf '\n'
if [ "$FAILURES" -eq 0 ]; then
  printf 'all resolve-root cases passed\n'
  exit 0
else
  printf '%s resolve-root case(s) failed\n' "$FAILURES"
  exit 1
fi
