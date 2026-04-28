# Destructive Operations

The CLI has three destructive operations:

- `wikijs delete <id>` removes a page.
- `wikijs assets delete <id>` removes an uploaded asset.
- `wikijs revert <page-id> <version-id>` overwrites live page content with an older version.

Risk review before changing these commands:

- Keep confirmation required by default.
- Keep `--force` explicit and command-local.
- Keep human prompts on stderr so stdout remains script-safe.
- Keep JSON success output structured when `--format json` is used.
- Add or update tests for cancellation, forced execution, and JSON output.
- Avoid broad selectors for destructive actions unless a future plan adds a separate dry-run and preview flow.
