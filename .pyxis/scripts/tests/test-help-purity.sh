#!/usr/bin/env bash
# test-help-purity.sh — the /help skill's derivation-purity conformance test
# (ecosystem-help-v1 #DINV-1/#DINV-3, prd:ecosystem-help/FR-4, plan T2).
#
# The skill derives every answer live from the routing surfaces and holds no
# catalog of its own. This test proves that mechanically, three arms:
#   a. scan     — no live capability name (a skill dir name) appears in help's
#                 own two files (#DINV-1: the catalog is never a kept list).
#   b. fixture  — the guide derives by scanning `.pyxis/skills/*/SKILL.md`, so a
#                 newly-dropped skill is picked up with zero edits to help's
#                 files (FR-4's fit criterion).
#   c. no-write — help's files carry no positive write/cache/persist
#                 instruction (#DINV-3: the skill is stateless).
# The assertion inputs (skill names) are harvested live from the surface at run
# time — never a list kept in this file (AP-27). Arms a and c each carry a
# self-verifying red-run: the detector is run against a planted temp copy and
# must fire, proving the check catches a real violation.
set -u
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SKILL="$REPO_ROOT/.pyxis/skills/help/SKILL.md"
GUIDE="$REPO_ROOT/.pyxis/authoring/help.md"
PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); printf 'ok   - %s\n' "$1"; }
bad() { FAIL=$((FAIL+1)); printf 'FAIL - %s\n' "$1"; }

for f in "$SKILL" "$GUIDE"; do
  [ -f "$f" ] || { bad "help file missing: $f"; echo "$FAIL failure(s)"; exit 1; }
done

# Harvest the live capability names: every skill directory except help's own.
# This is the "derived assertion list" — recomputed each run from the surface.
harvest_skill_names() {
  for d in "$REPO_ROOT"/.pyxis/skills/*/; do
    name="$(basename "$d")"
    [ "$name" = "help" ] && continue
    printf '%s\n' "$name"
  done
}

# scan_literals <file...> — echo any harvested skill name found in the files,
# matched as a whole token (kebab identifiers, non-word bounded). Silent = pure.
scan_literals() {
  local hits="" name
  while IFS= read -r name; do
    if grep -qE "(^|[^A-Za-z0-9_-])${name}([^A-Za-z0-9_-]|$)" "$@" 2>/dev/null; then
      hits="$hits $name"
    fi
  done <<EOF
$(harvest_skill_names)
EOF
  printf '%s' "$hits"
}

# scan_write_instructions <file...> — echo any positive write/cache/persist
# instruction (verb + a to/in/as/into target). A prohibition ("Never write…",
# "writes nothing — no cache") carries no such target and does not match.
scan_write_instructions() {
  grep -niE '(write|writes|save|saves|cache|caches|persist|persists|store|stores)[^.]{0,40}(to|in|as|into) ' "$@" 2>/dev/null || true
}

# --- Arm a: the purity scan on the real tree ---------------------------------
HITS="$(scan_literals "$SKILL" "$GUIDE")"
if [ -z "$HITS" ]; then ok "scan: no capability literal in help's own files (#DINV-1)"
else bad "scan: help's files hardcode capability name(s):$HITS"; fi

# Red-run: plant a skill name into a temp copy; the detector must fire.
TMPD="$(mktemp -d "${TMPDIR:-/tmp}/pyxis-help-purity.XXXXXXXX")"; trap 'rm -rf "$TMPD"' EXIT
PLANT_NAME="$(harvest_skill_names | sed -n '1p')"
cp "$GUIDE" "$TMPD/guide.md"
printf '\nRun the %s skill first, then me.\n' "$PLANT_NAME" >> "$TMPD/guide.md"
if [ -n "$(scan_literals "$TMPD/guide.md")" ]; then
  ok "scan red-run: a planted capability literal is detected"
else bad "scan red-run: planted literal '$PLANT_NAME' slipped past the scan"; fi

# --- Arm b: the add-a-skill fixture ------------------------------------------
# The guide must derive by scanning the skills glob (not a kept list).
if grep -qF 'skills/*/SKILL.md' "$GUIDE"; then
  ok "fixture: the guide derives by scanning .pyxis/skills/*/SKILL.md"
else bad "fixture: the guide does not name the skills-tree scan — derivation may be a kept list"; fi

# Snapshot help's own files, then drop a throwaway skill into a sandbox copy of
# the skills tree. The glob the guide names must pick the throwaway up, and
# help's own files must be byte-identical afterward — proving a new skill
# surfaces with zero help-side edits (FR-4's fit criterion).
cp "$SKILL" "$TMPD/skill.before"; cp "$GUIDE" "$TMPD/guide.before"
SANDBOX="$TMPD/skills"; mkdir -p "$SANDBOX"
cp -R "$REPO_ROOT"/.pyxis/skills/. "$SANDBOX/"
mkdir -p "$SANDBOX/zz-throwaway-probe"
printf -- '---\nname: zz-throwaway-probe\ndescription: a fixture skill.\n---\n' > "$SANDBOX/zz-throwaway-probe/SKILL.md"
if ls "$SANDBOX"/*/SKILL.md 2>/dev/null | grep -q 'zz-throwaway-probe/SKILL.md'; then
  ok "fixture: a newly-dropped skill is picked up by the derivation scan"
else bad "fixture: the derivation scan did not pick up the new skill"; fi
if cmp -s "$SKILL" "$TMPD/skill.before" && cmp -s "$GUIDE" "$TMPD/guide.before"; then
  ok "fixture: surfacing a new skill required zero edits to help's own files (FR-4)"
else bad "fixture: help's own files changed during the fixture"; fi

# --- Arm c: statelessness (no positive write instruction) --------------------
WI="$(scan_write_instructions "$SKILL" "$GUIDE")"
if [ -z "$WI" ]; then ok "no-write: help's files carry no write/cache/persist instruction (#DINV-3)"
else bad "no-write: help's files instruct a write/cache/persist:"$'\n'"$WI"; fi

# Red-run: plant a write instruction into a temp copy; the detector must fire.
cp "$GUIDE" "$TMPD/guide2.md"
printf '\nWrite the derived catalog to a cache file so later invocations skip the walk.\n' >> "$TMPD/guide2.md"
if [ -n "$(scan_write_instructions "$TMPD/guide2.md")" ]; then
  ok "no-write red-run: a planted write instruction is detected"
else bad "no-write red-run: planted write instruction slipped past the check"; fi

echo "$PASS pass, $FAIL fail"
[ "$FAIL" -eq 0 ]
