#!/usr/bin/env bash
set +e

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 2
BIN="$ROOT/wikijs"
URL="$(awk -F= '$1=="URL"{print $2}' .env)"
TOKEN="$(awk -F= '$1=="TOKEN"{print $2}' .env)"
if [ -z "$TOKEN" ]; then TOKEN="$(awk -F: '$1=="TOKEN"{print $2}' .env)"; fi
WORK="$(mktemp -d /private/tmp/wikijs-invalid.XXXXXX)"
export WIKIJS_URL="$URL"
export WIKIJS_API_TOKEN="$TOKEN"
export XDG_CONFIG_HOME="$WORK/xdg"

PASS=0
FAIL=0
FAILS=()
LOG="$WORK/invalid.log"
: > "$LOG"

record_pass() { PASS=$((PASS+1)); printf 'PASS %s\n' "$1"; }
record_fail() { FAIL=$((FAIL+1)); FAILS+=("$1"); printf 'FAIL %s\n' "$1"; }

expect_fail() {
  local name="$1"
  local want="$2"
  shift 2
  local outfile="$WORK/${name//[^A-Za-z0-9_.-]/_}.out"
  printf '\n### %s\n$ %s\n' "$name" "$*" >> "$LOG"
  "$@" >"$outfile" 2>&1
  local status=$?
  cat "$outfile" >> "$LOG"
  printf '\n[exit %d]\n' "$status" >> "$LOG"
  if [ $status -ne 0 ] && grep -Eiq "$want" "$outfile"; then
    record_pass "$name"
  else
    record_fail "$name expected failure matching /$want/ but exit=$status output=$(tr '\r\n' ' ' < "$outfile" | cut -c1-220)"
  fi
}

expect_success() {
  local name="$1"
  shift
  local outfile="$WORK/${name//[^A-Za-z0-9_.-]/_}.out"
  printf '\n### %s\n$ %s\n' "$name" "$*" >> "$LOG"
  "$@" >"$outfile" 2>&1
  local status=$?
  cat "$outfile" >> "$LOG"
  printf '\n[exit %d]\n' "$status" >> "$LOG"
  if [ $status -eq 0 ]; then
    record_pass "$name"
  else
    record_fail "$name expected success but exit=$status output=$(tr '\r\n' ' ' < "$outfile" | cut -c1-220)"
  fi
}

expect_sh_fail() {
  local name="$1"
  local want="$2"
  local script="$3"
  local outfile="$WORK/${name//[^A-Za-z0-9_.-]/_}.out"
  printf '\n### %s\n$ %s\n' "$name" "$script" >> "$LOG"
  bash -lc "$script" >"$outfile" 2>&1
  local status=$?
  cat "$outfile" >> "$LOG"
  printf '\n[exit %d]\n' "$status" >> "$LOG"
  if [ $status -ne 0 ] && grep -Eiq "$want" "$outfile"; then
    record_pass "$name"
  else
    record_fail "$name expected failure matching /$want/ but exit=$status output=$(tr '\r\n' ' ' < "$outfile" | cut -c1-220)"
  fi
}

expect_sh_success() {
  local name="$1"
  local script="$2"
  local outfile="$WORK/${name//[^A-Za-z0-9_.-]/_}.out"
  printf '\n### %s\n$ %s\n' "$name" "$script" >> "$LOG"
  bash -lc "$script" >"$outfile" 2>&1
  local status=$?
  cat "$outfile" >> "$LOG"
  printf '\n[exit %d]\n' "$status" >> "$LOG"
  if [ $status -eq 0 ]; then
    record_pass "$name"
  else
    record_fail "$name expected success but exit=$status output=$(tr '\r\n' ' ' < "$outfile" | cut -c1-220)"
  fi
}

bad_cfg="$WORK/bad.json"
missing_cfg="$WORK/missing.json"
invalid_backup="$WORK/invalid-backup.json"
bad_json="$WORK/bad-json.json"
empty_dir="$WORK/empty-dir"
mkdir -p "$empty_dir"
printf '{"url":' > "$bad_json"
printf '{"apiToken":"token"}' > "$bad_cfg"
printf '{"version":1,"pages":"not-array"}' > "$invalid_backup"

expect_fail root-unknown "unknown command" "$BIN" no-such-command
expect_fail bad-no-color-flag "unknown flag" "$BIN" --no-colors health
expect_fail unsupported-format "unsupported format" "$BIN" --format xml list
expect_fail missing-config "Config error|config file not found" env -u WIKIJS_URL -u WIKIJS_API_TOKEN -u WIKIJS_CONFIG "$BIN" --config "$missing_cfg" health
expect_fail invalid-config "Config error|missing \"url\"" env -u WIKIJS_URL -u WIKIJS_API_TOKEN -u WIKIJS_CONFIG "$BIN" --config "$bad_cfg" health
expect_fail malformed-config "parse config|Config error" env -u WIKIJS_URL -u WIKIJS_API_TOKEN -u WIKIJS_CONFIG "$BIN" --config "$bad_json" health
expect_fail bad-auth "Authentication failed|rejected" env WIKIJS_URL="$URL" WIKIJS_API_TOKEN="bad-token" "$BIN" health
expect_fail bad-url "unsupported protocol|no such host|connect|invalid|http" env WIKIJS_URL="http://127.0.0.1:1" WIKIJS_API_TOKEN="$TOKEN" "$BIN" health

expect_fail list-bad-limit "invalid argument|parse" "$BIN" list --limit abc
expect_fail list-unknown-flag "unknown flag" "$BIN" list --bogus
expect_fail search-missing-arg "accepts 1 arg|requires" "$BIN" search
expect_fail search-too-many "accepts 1 arg" "$BIN" search one two
expect_fail search-bad-limit "invalid argument|parse" "$BIN" search home --limit nope
expect_fail get-missing-arg "accepts 1 arg|requires" "$BIN" get
expect_fail get-missing-page "not found|does not exist|Page not found|GraphQL" "$BIN" get /definitely-missing-codex-page
expect_fail get-too-many "accepts 1 arg" "$BIN" get 1 2

expect_fail create-missing-args "accepts 2 arg|requires" "$BIN" create /x
expect_fail create-too-many-args "accepts 2 arg" "$BIN" create /x Title extra
expect_fail create-invalid-path "invalid characters|validation" "$BIN" create "/bad<path" "Bad" --content body
expect_fail create-conflicting-content "use only one of --file, --content, or --stdin" "$BIN" create /codex-invalid/conflict Conflict --content body --file "$bad_cfg"
expect_fail create-missing-file "no such file|cannot find" "$BIN" create /codex-invalid/missing Missing --file "$WORK/nope.md"
expect_sh_fail create-empty-stdin "use only one|accepts|content" "printf '' | '$BIN' create /codex-invalid/stdin Empty --stdin --content body"

expect_fail update-missing-arg "accepts 1 arg|requires" "$BIN" update
expect_fail update-bad-id "invalid id" "$BIN" update abc --content body
expect_fail update-zero-id "invalid id" "$BIN" update 0 --content body
expect_fail update-conflicting-content "use only one of --file, --content, or --stdin" "$BIN" update 1 --content body --file "$bad_cfg"
expect_fail update-conflicting-published "cannot both be set" "$BIN" update 1 --published --unpublished
expect_fail update-missing-file "no such file|cannot find" "$BIN" update 1 --file "$WORK/nope.md"

expect_fail move-missing-args "accepts 2 arg|requires" "$BIN" move 1
expect_fail move-bad-id "invalid id" "$BIN" move abc /new-path
expect_fail move-invalid-path "invalid characters|validation" "$BIN" move 1 "/bad<path"
expect_fail delete-missing-arg "accepts 1 arg|requires" "$BIN" delete
expect_fail delete-bad-id "invalid id" "$BIN" delete abc --force
expect_sh_fail delete-cancelled "cancelled" "printf 'no\n' | '$BIN' delete 1"

expect_fail tag-missing-args "accepts 3 arg|requires" "$BIN" tag 1 add
expect_fail tag-bad-id "invalid id" "$BIN" tag abc add docs
expect_fail tag-empty-tags "at least one tag" "$BIN" tag 1 add ",,,"
expect_fail tag-unsupported-op "unsupported tag operation" "$BIN" tag 1 bogus docs

expect_fail grep-missing-arg "accepts 1 arg|requires" "$BIN" grep
expect_fail grep-empty-pattern "pattern must not be empty" "$BIN" grep ""
expect_fail grep-invalid-regex "missing|error parsing regexp|invalid" "$BIN" grep "[" --regex
expect_fail grep-bad-limit "invalid argument|parse" "$BIN" grep TODO --limit abc

expect_fail versions-missing-arg "accepts 1 arg|requires" "$BIN" versions
expect_fail versions-bad-id "invalid id" "$BIN" versions abc
expect_fail revert-missing-args "accepts 2 arg|requires" "$BIN" revert 1
expect_fail revert-bad-page-id "invalid id" "$BIN" revert abc 1 --force
expect_fail revert-bad-version-id "invalid id" "$BIN" revert 1 abc --force
expect_sh_fail revert-cancelled "cancelled" "printf 'no\n' | '$BIN' revert 1 1"

expect_success asset-parent-help "$BIN" asset
expect_fail asset-list-bad-limit "invalid argument|parse" "$BIN" asset list --limit abc
expect_fail asset-upload-missing-arg "accepts 1 arg|requires" "$BIN" asset upload
expect_fail asset-upload-missing-file "no such file|cannot find" "$BIN" asset upload "$WORK/nope.png"
expect_fail asset-delete-missing-arg "accepts 1 arg|requires" "$BIN" asset delete
expect_fail asset-delete-bad-id "invalid id" "$BIN" asset delete abc --force
expect_sh_fail asset-delete-cancelled "cancelled" "printf 'no\n' | '$BIN' asset delete 1"

expect_fail lint-missing-arg "accepts 1 arg|requires" "$BIN" lint
expect_fail lint-missing-file "no such file|cannot find" "$BIN" lint "$WORK/nope.md"
expect_fail backup-bad-output-dir "is a directory|permission denied|no such file" "$BIN" backup --output "$empty_dir"
expect_fail restore-missing-arg "accepts 1 arg|requires" "$BIN" restore-backup
expect_fail restore-missing-file "no such file|cannot find" "$BIN" restore-backup "$WORK/nope.json"
expect_fail restore-bad-json "cannot unmarshal|invalid character|json" "$BIN" restore-backup "$invalid_backup" --dry-run

expect_fail export-missing-arg "accepts 1 arg|requires" "$BIN" export
expect_fail export-bad-format "unsupported file format" "$BIN" export "$WORK/export" --file-format xml
expect_fail sync-bad-format "unsupported file format" "$BIN" sync --output "$WORK/sync" --file-format xml
expect_fail sync-no-output "sync output path is required" env WIKIJS_CONFIG="$WORK/no-sync-config.json" WIKIJS_URL="$URL" WIKIJS_API_TOKEN="$TOKEN" "$BIN" sync

expect_fail diff-missing-arg "requires at least 1 arg|accepts between" "$BIN" diff
expect_fail diff-bad-page-id "invalid id" "$BIN" diff abc
expect_fail diff-bad-version-id "invalid id" "$BIN" diff 1 abc
expect_fail diff-too-many-args "accepts between 1 and 3 arg" "$BIN" diff 1 2 3 4
expect_fail clone-missing-args "accepts 2 arg|requires" "$BIN" clone 1
expect_fail clone-missing-source "not found|does not exist|GraphQL" "$BIN" clone /definitely-missing-codex-page /codex-invalid/clone
expect_fail clone-invalid-destination "invalid characters|validation" "$BIN" clone 1 "/bad<path"

expect_fail validate-missing-arg "requires an id/path argument" "$BIN" validate
expect_fail validate-all-with-arg "use either --all" "$BIN" validate --all 1
expect_fail validate-missing-page "not found|does not exist|GraphQL" "$BIN" validate /definitely-missing-codex-page
expect_fail check-links-unknown-flag "unknown flag" "$BIN" check-links --bogus

expect_fail replace-missing-args "accepts 2 arg|requires" "$BIN" replace old
expect_fail replace-empty-old "old text must not be empty" "$BIN" replace "" new --dry-run
expect_fail replace-invalid-regex "missing|error parsing regexp|invalid" "$BIN" replace "[" new --regex --dry-run
expect_sh_fail replace-cancelled "cancelled" "printf 'no\n' | '$BIN' replace old new"

expect_fail bulk-create-missing-arg "accepts 1 arg|requires" "$BIN" bulk-create
expect_fail bulk-create-missing-dir "no such file|cannot find" "$BIN" bulk-create "$WORK/no-dir"
expect_success bulk-create-empty-dir "$BIN" bulk-create "$empty_dir" --dry-run
expect_fail bulk-update-missing-arg "accepts 1 arg|requires" "$BIN" bulk-update
expect_fail bulk-update-missing-dir "no such file|cannot find" "$BIN" bulk-update "$WORK/no-dir"

expect_success template-parent-help "$BIN" template
expect_fail template-create-missing-arg "accepts 1 arg|requires" "$BIN" template create
expect_fail template-create-no-content "template content is required" "$BIN" template create codex-empty-template
expect_fail template-create-conflicting-content "use only one of --file, --content, or --stdin" "$BIN" template create codex-conflict --content body --file "$bad_cfg"
expect_fail template-create-bad-name "path separators" "$BIN" template create "bad/name" --content body
expect_fail template-show-missing "no such file|cannot find" "$BIN" template show definitely-missing-template
expect_fail template-delete-missing "no such file|cannot find" "$BIN" template delete definitely-missing-template --force
expect_sh_fail template-delete-cancelled "cancelled" "printf 'no\n' | '$BIN' template delete definitely-missing-template"

expect_success completion-parent-help "$BIN" completion
expect_success completion-unknown-shell-shows-help "$BIN" completion tcsh
expect_success help-unknown-topic-shows-root-help "$BIN" help no-such-command
expect_sh_success shell-unknown-command-exits-cleanly "printf 'no-such-command\nexit\n' | '$BIN' shell"

printf 'SUMMARY pass=%d fail=%d log=%s\n' "$PASS" "$FAIL" "$LOG"
if [ "$FAIL" -gt 0 ]; then
  printf 'FAILURES:\n'
  printf '%s\n' "${FAILS[@]}"
  exit 1
fi
