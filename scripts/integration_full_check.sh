#!/usr/bin/env bash
set +e
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 2
BIN="$ROOT/wikijs"
URL="$(awk -F= '$1=="URL"{print $2}' .env)"
TOKEN="$(awk -F= '$1=="TOKEN"{print $2}' .env)"
if [ -z "$TOKEN" ]; then TOKEN="$(awk -F: '$1=="TOKEN"{print $2}' .env)"; fi
RUN="codex-it-$(date +%s)"
PREFIX="$RUN"
WORK="$(mktemp -d /private/tmp/wikijs-it.XXXXXX)"
export XDG_CONFIG_HOME="$WORK/xdg"
export WIKIJS_URL="$URL"
export WIKIJS_API_TOKEN="$TOKEN"
PASS=0
FAIL=0
XFAIL=0
FAILS=()
LOG="$WORK/full.log"
: > "$LOG"

say() { printf '%s\n' "$*" | tee -a "$LOG"; }
run() {
  local name="$1"; shift
  local outfile="$WORK/${name//[^A-Za-z0-9_.-]/_}.out"
  printf '\n### %s\n$ %s\n' "$name" "$*" >> "$LOG"
  "$@" >"$outfile" 2>&1
  local status=$?
  cat "$outfile" >> "$LOG"
  printf '\n[exit %d]\n' "$status" >> "$LOG"
  if [ $status -eq 0 ]; then
    PASS=$((PASS+1)); say "PASS $name"
  else
    FAIL=$((FAIL+1)); FAILS+=("$name (exit $status): $(tr '\r\n' ' ' < "$outfile" | cut -c1-220)"); say "FAIL $name"
  fi
  return $status
}
run_sh() {
  local name="$1"; shift
  local script="$1"
  local outfile="$WORK/${name//[^A-Za-z0-9_.-]/_}.out"
  printf '\n### %s\n$ %s\n' "$name" "$script" >> "$LOG"
  bash -lc "$script" >"$outfile" 2>&1
  local status=$?
  cat "$outfile" >> "$LOG"
  printf '\n[exit %d]\n' "$status" >> "$LOG"
  if [ $status -eq 0 ]; then
    PASS=$((PASS+1)); say "PASS $name"
  else
    FAIL=$((FAIL+1)); FAILS+=("$name (exit $status): $(tr '\r\n' ' ' < "$outfile" | cut -c1-220)"); say "FAIL $name"
  fi
  return $status
}
expect_fail() {
  local name="$1"; local want="$2"; shift 2
  local outfile="$WORK/${name//[^A-Za-z0-9_.-]/_}.out"
  printf '\n### %s (expected failure)\n$ %s\n' "$name" "$*" >> "$LOG"
  "$@" >"$outfile" 2>&1
  local status=$?
  cat "$outfile" >> "$LOG"
  printf '\n[exit %d]\n' "$status" >> "$LOG"
  if [ $status -ne 0 ] && grep -q "$want" "$outfile"; then
    XFAIL=$((XFAIL+1)); say "XFAIL $name"
  else
    FAIL=$((FAIL+1)); FAILS+=("$name expected failure containing '$want' but exit=$status: $(tr '\r\n' ' ' < "$outfile" | cut -c1-220)"); say "FAIL $name"
  fi
}
json_cmd() { "$BIN" --format json "$@"; }

cleanup() {
  say "Cleanup for prefix $PREFIX"
  json_cmd list --limit 0 > "$WORK/pages.json" 2>>"$LOG"
  jq -r --arg p "$PREFIX" '.[] | select(.path | startswith($p)) | .id' "$WORK/pages.json" | sort -rn | while read -r id; do
    [ -n "$id" ] && "$BIN" delete "$id" --force >>"$LOG" 2>&1
  done
  json_cmd asset list > "$WORK/assets.json" 2>>"$LOG"
  jq -r --arg f "${RUN}-asset.png" '.[] | select(.filename == $f) | .id' "$WORK/assets.json" | while read -r id; do
    [ -n "$id" ] && "$BIN" asset delete "$id" --force >>"$LOG" 2>&1
  done
}
trap cleanup EXIT

say "Integration prefix: $PREFIX"
say "Workspace: $WORK"

# Help and completion surface.
run help-root "$BIN" --help
for cmd in health list search get create update move delete tags tag info grep stats versions revert asset tree lint backup restore-backup export sync check-links diff clone validate bulk-create bulk-update template shell; do
  run "help-$cmd" "$BIN" "$cmd" --help
done
for shell in bash zsh fish powershell; do
  run "completion-$shell" "$BIN" completion "$shell"
done

# Local-only commands.
printf '# Lint title\n\nBody\n' > "$WORK/lint.md"
run lint "$BIN" lint "$WORK/lint.md"
run lint-json "$BIN" --format json lint "$WORK/lint.md"
run template-list-empty "$BIN" template list
run template-create-content "$BIN" template create "$RUN-template" --content '# {{title}} at {{path}}'
run template-show "$BIN" template show "$RUN-template"
run template-list "$BIN" template list
run template-delete "$BIN" template delete "$RUN-template" --force
run_sh shell-basic "printf 'health\nlist --limit 1\nexit\n' | '$BIN' shell"

# Baseline read commands.
run health "$BIN" health
run health-no-color "$BIN" --no-color health
run_sh health-no-color-no-ansi "'$BIN' --no-color health > '$WORK/no-color-health.out' && ! LC_ALL=C grep -q $'\033' '$WORK/no-color-health.out'"
run health-json "$BIN" --format json health
run health-verbose "$BIN" --verbose health
run list "$BIN" list --limit 5
run list-json "$BIN" --format json list --limit 0
run search-home "$BIN" search home --limit 5
run search-json "$BIN" --format json search home --limit 5
run tags "$BIN" tags
run tags-json "$BIN" --format json tags
run stats "$BIN" stats
run stats-json "$BIN" --format json stats
run stats-detailed "$BIN" stats --detailed
run asset-list "$BIN" asset list
run asset-list-json "$BIN" --format json asset list
run asset-list-limit "$BIN" asset list --limit 2
run tree "$BIN" tree
run tree-json "$BIN" --format json tree

# Create a reusable set of pages.
printf '# File Content\n\nNeedleAlpha from file.\n[Home](/home)\n' > "$WORK/source.md"
source_json="$WORK/source.json"
json_cmd create "/$PREFIX/source" "Codex Source" --content '# Codex Source

NeedleAlpha initial content.
[Home](/home)' --description "integration source" --tag "codex,$RUN" --draft --private > "$source_json" 2>>"$LOG"
if [ $? -eq 0 ]; then PASS=$((PASS+1)); say "PASS create-json-content"; else FAIL=$((FAIL+1)); FAILS+=("create-json-content failed: $(cat "$source_json" | tr '\r\n' ' ' | cut -c1-220)"); say "FAIL create-json-content"; fi
SOURCE_ID="$(jq -r '.id // empty' "$source_json")"
run get-id "$BIN" get "$SOURCE_ID"
run get-path "$BIN" get "/$PREFIX/source"
run get-raw "$BIN" get "$SOURCE_ID" --raw
run get-raw-metadata "$BIN" get "$SOURCE_ID" --raw --metadata
run get-json "$BIN" --format json get "$SOURCE_ID"
run info "$BIN" info "$SOURCE_ID"
run info-path "$BIN" info "/$PREFIX/source"
run update-file "$BIN" update "$SOURCE_ID" --file "$WORK/source.md"
run update-title-tags "$BIN" update "$SOURCE_ID" --title "Codex Source Updated" --description "updated desc" --tags "codex,updated,$RUN"
run update-content "$BIN" update "$SOURCE_ID" --content '# Updated Content

NeedleAlpha updated content.
[Home](/home)'
run update-unpublished "$BIN" update "$SOURCE_ID" --unpublished
run update-published "$BIN" update "$SOURCE_ID" --published
run tag-add "$BIN" tag "$SOURCE_ID" add extra-tag
run tag-remove "$BIN" tag "$SOURCE_ID" remove extra-tag
run tag-set "$BIN" tag "$SOURCE_ID" set "codex,$RUN"
run move "$BIN" move "$SOURCE_ID" "/$PREFIX/moved-source"
run clone "$BIN" clone "$SOURCE_ID" "/$PREFIX/clone" --with-tags --title "Codex Clone"

stdin_json="$WORK/stdin.json"
printf '# Stdin Page\n\nNeedleAlpha from stdin.\n' | "$BIN" --format json create "/$PREFIX/stdin" "Codex Stdin" --stdin > "$stdin_json" 2>>"$LOG"
if [ $? -eq 0 ]; then PASS=$((PASS+1)); say "PASS create-stdin"; else FAIL=$((FAIL+1)); FAILS+=("create-stdin failed: $(cat "$stdin_json" | tr '\r\n' ' ' | cut -c1-220)"); say "FAIL create-stdin"; fi
STDIN_ID="$(jq -r '.id // empty' "$stdin_json")"
run_sh update-stdin "printf '# Updated via stdin\n\nNeedleAlpha stdin update.\n' | '$BIN' update '$STDIN_ID' --stdin"

run template-create-for-page "$BIN" template create "$RUN-template" --content '# {{title}}

Template body for {{path}} with NeedleAlpha.'
run create-template "$BIN" create "/$PREFIX/from-template" "Codex Template Page" --template "$RUN-template"
run template-delete-for-page "$BIN" template delete "$RUN-template" --force
run create-file "$BIN" create "/$PREFIX/from-file" "Codex File Page" --file "$WORK/source.md" --tag "codex,$RUN"

# Discovery and content commands after pages exist.
run list-prefix-json "$BIN" --format json list --limit 0
run search-codex "$BIN" search Codex --limit 10
run grep "$BIN" grep NeedleAlpha --path "$PREFIX" --limit 5
run grep-json-regex "$BIN" --format json grep 'Needle[A-Za-z]+' --regex --case-sensitive --path "$PREFIX" --limit 3
run replace-dry-no-match "$BIN" replace "fake1234" "newfake" --dry-run --path "$PREFIX"
run replace-dry-match "$BIN" replace "NeedleAlpha" "NeedleBeta" --dry-run --path "$PREFIX"
run replace-force "$BIN" replace "NeedleAlpha" "NeedleBeta" --force --path "$PREFIX"
run replace-regex-dry "$BIN" replace "Needle(Beta)" "Needle$1" --regex --dry-run --path "$PREFIX"
run check-links-good "$BIN" check-links --path "$PREFIX"
run validate-one-good "$BIN" validate "$SOURCE_ID"
run validate-all-prefix "$BIN" validate --all --path "$PREFIX"

# Broken link/image expected failures.
broken_json="$WORK/broken.json"
json_cmd create "/$PREFIX/broken" "Codex Broken" --content '# Broken

[Missing](/missing-codex-page)
![Missing Image](/missing-codex-image.png)' > "$broken_json" 2>>"$LOG"
if [ $? -eq 0 ]; then PASS=$((PASS+1)); say "PASS create-broken"; else FAIL=$((FAIL+1)); FAILS+=("create-broken failed: $(cat "$broken_json" | tr '\r\n' ' ' | cut -c1-220)"); say "FAIL create-broken"; fi
BROKEN_ID="$(jq -r '.id // empty' "$broken_json")"
expect_fail check-links-broken "broken internal links" "$BIN" check-links --path "$PREFIX"
expect_fail validate-broken "validation failed" "$BIN" validate "$BROKEN_ID"
expect_fail validate-all-broken "validation failed" "$BIN" validate --all --path "$PREFIX"

# Versions, diff, revert.
run versions "$BIN" versions "$SOURCE_ID"
run versions-json "$BIN" --format json versions "$SOURCE_ID"
json_cmd versions "$SOURCE_ID" > "$WORK/versions.json" 2>>"$LOG"
VCOUNT="$(jq 'length' "$WORK/versions.json")"
if [ "${VCOUNT:-0}" -ge 2 ]; then
  VNEW="$(jq -r '.[0].versionId' "$WORK/versions.json")"
  VOLD="$(jq -r '.[length-1].versionId' "$WORK/versions.json")"
  run diff-default "$BIN" diff "$SOURCE_ID"
  run diff-one-version "$BIN" diff "$SOURCE_ID" "$VOLD"
  run diff-two-versions "$BIN" diff "$SOURCE_ID" "$VOLD" "$VNEW"
  run diff-json "$BIN" --format json diff "$SOURCE_ID" "$VOLD" "$VNEW"
  run revert "$BIN" revert "$SOURCE_ID" "$VOLD" --force
else
  FAIL=$((FAIL+1)); FAILS+=("versions-json returned fewer than two versions for $SOURCE_ID"); say "FAIL versions-count"
fi

# Backup, restore, export, sync.
run backup-stdout "$BIN" backup --output -
run backup-file "$BIN" backup --output "$WORK/backup.json"
run backup-file-json "$BIN" --format json backup --output "$WORK/backup2.json"
cat > "$WORK/restore.json" <<JSON
{"version":1,"exportedAt":"2026-04-28T00:00:00Z","pages":[{"path":"$PREFIX/restored","title":"Codex Restored","content":"# Restored\n\nNeedleBeta restored.","locale":"en","isPublished":true}]}
JSON
run restore-dry "$BIN" restore-backup "$WORK/restore.json" --dry-run
run restore-create "$BIN" restore-backup "$WORK/restore.json"
run restore-skip-existing "$BIN" restore-backup "$WORK/restore.json" --skip-existing
run restore-force "$BIN" restore-backup "$WORK/restore.json" --force
run export-md "$BIN" export "$WORK/export-md" --path "$PREFIX"
run export-json "$BIN" --format json export "$WORK/export-json" --file-format json --path "$PREFIX"
run sync-md "$BIN" sync --output "$WORK/sync-md" --path "$PREFIX"
run sync-json "$BIN" --format json sync --output "$WORK/sync-json" --file-format json --path "$PREFIX"
printf 'stale' > "$WORK/sync-md/stale.md"
run sync-delete "$BIN" sync --output "$WORK/sync-md" --path "$PREFIX" --delete

# Bulk operations.
mkdir -p "$WORK/bulk/sub"
printf '# Bulk One\n\nNeedleBeta bulk one.\n' > "$WORK/bulk/one.md"
printf '# Bulk Two\n\nNeedleBeta bulk two.\n' > "$WORK/bulk/sub/two.md"
run bulk-create-dry "$BIN" bulk-create "$WORK/bulk" --path-prefix "$PREFIX/bulk-dry" --dry-run
run bulk-create "$BIN" bulk-create "$WORK/bulk" --path-prefix "$PREFIX/bulk" --tag "codex,$RUN"
printf '# Bulk One Updated\n\nNeedleBeta bulk one updated.\n' > "$WORK/bulk/one.md"
run bulk-update-dry "$BIN" bulk-update "$WORK/bulk" --path-prefix "$PREFIX/bulk" --dry-run
run bulk-update "$BIN" bulk-update "$WORK/bulk" --path-prefix "$PREFIX/bulk"
printf '# Missing\n' > "$WORK/bulk/missing.md"
run bulk-update-skip-missing "$BIN" bulk-update "$WORK/bulk" --path-prefix "$PREFIX/bulk" --skip-missing

# Asset upload/list/delete.
printf 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=' | base64 --decode > "$WORK/asset.png" 2>/dev/null || printf 'fake image' > "$WORK/asset.png"
run asset-upload "$BIN" asset upload "$WORK/asset.png" --rename "${RUN}-asset.png"
json_cmd asset list > "$WORK/assets-after-upload.json" 2>>"$LOG"
ASSET_ID="$(jq -r --arg f "${RUN}-asset.png" '.[] | select(.filename == $f) | .id' "$WORK/assets-after-upload.json" | head -1)"
if [ -n "$ASSET_ID" ]; then
  run asset-delete "$BIN" asset delete "$ASSET_ID" --force
else
  FAIL=$((FAIL+1)); FAILS+=("uploaded asset not found in asset list"); say "FAIL asset-upload-find"
fi

# Delete command on a created page.
if [ -n "$BROKEN_ID" ]; then run delete-broken "$BIN" delete "$BROKEN_ID" --force; fi

say "SUMMARY pass=$PASS expected_fail=$XFAIL fail=$FAIL log=$LOG"
if [ $FAIL -gt 0 ]; then
  printf 'FAILURES:\n' | tee -a "$LOG"
  printf '%s\n' "${FAILS[@]}" | tee -a "$LOG"
  exit 1
fi
exit 0
