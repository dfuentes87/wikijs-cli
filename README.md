# wikijs-cli

A Go command-line client for Wiki.js 2.x.

Forked from [hopyky/wikijs-cli](https://github.com/hopyky/wikijs-cli) which was written in JavaScript.

## Install

```bash
go install github.com/dfuentes87/wikijs-cli/cmd/wikijs@latest
```

For local development:

```bash
go build ./cmd/wikijs
go test ./...
```

Typical release artifact names:

- `wikijs_darwin_arm64`
- `wikijs_darwin_amd64`
- `wikijs_linux_amd64`
- `wikijs_linux_arm64`
- `wikijs_windows_amd64.exe`

## Configuration

Copy the example config and edit it:

```bash
mkdir -p ~/.config
cp config/wikijs.example.json ~/.config/wikijs.json
```

Default paths:

- macOS/Linux: `~/.config/wikijs.json`
- Windows: `%AppData%\wikijs\config.json`

You can override the config path with `--config` or `WIKIJS_CONFIG`.
If `WIKIJS_URL` and `WIKIJS_API_TOKEN` are set, the CLI can run without a
config file.

Environment variables override values from the JSON file:

- `WIKIJS_URL`
- `WIKIJS_API_TOKEN`
- `WIKIJS_DEFAULT_LOCALE`
- `WIKIJS_DEFAULT_EDITOR`

## Global Flags

```bash
wikijs --config ./wikijs.json <command>
wikijs --format json <command>
wikijs --verbose <command>
wikijs --debug <command>
wikijs --rate-limit 500 <command>
```

Supported output formats are `table` and `json`.
`--verbose` logs request paths and HTTP statuses to stderr. `--debug` also logs
GraphQL variables with token-like fields redacted.

## Commands

```bash
wikijs version
wikijs health
wikijs list --limit 20 --locale en --tag docs
wikijs search "configuration"

wikijs get 1
wikijs get /docs/guide --raw
wikijs get 1 --raw --metadata

wikijs create /docs/intro "Introduction" --content "# Welcome"
wikijs create /docs/guide "Guide" --file ./guide.md --tag docs,guide
wikijs create /docs/readme "README" --stdin

wikijs update 1 --content "Updated content"
wikijs update 1 --file ./updated.md
wikijs update 1 --title "New title" --tags docs,important

wikijs move 1 /new/path
wikijs delete 1
wikijs delete 1 --force

wikijs tags
wikijs stats
wikijs versions 1
wikijs revert 1 5
wikijs revert 1 5 --force

wikijs backup --output backup.json
wikijs backup --output - --format json
wikijs restore-backup backup.json --dry-run
wikijs restore-backup backup.json --skip-existing
wikijs restore-backup backup.json --force

wikijs bulk-create ./pages --path-prefix /docs --tag imported
wikijs bulk-create ./pages --dry-run
wikijs bulk-update ./pages --path-prefix /docs
wikijs bulk-update ./pages --skip-missing

wikijs template list
wikijs template create doc --content "# {{title}}\n\nCreated: {{date}}"
wikijs template create doc --file ./template.md
wikijs template show doc
wikijs template delete doc --force
wikijs create /docs/new "New Page" --template doc

wikijs replace "old term" "new term" --dry-run
wikijs replace "old term" "new term" --path /docs --force
wikijs replace "old[0-9]+" "new" --regex --case-sensitive --force

wikijs assets list
wikijs assets list --folder /uploads --limit 100
wikijs assets upload ./image.png
wikijs assets upload ./image.png --rename hero.png
wikijs assets delete 42
wikijs assets delete 42 --force

wikijs tree
wikijs tree --locale en

wikijs lint ./document.md
wikijs lint ./document.md --format json

wikijs shell
```

When `--format json` is used, successful mutating commands return a structured
object with `success` and `action` fields.

## Shell Completion

Completion scripts are provided by Cobra:

```bash
wikijs completion bash
wikijs completion zsh
wikijs completion fish
wikijs completion powershell
```

Use `wikijs completion --help` for shell-specific installation instructions.

## API Compatibility

The CLI targets Wiki.js 2.x:

- GraphQL requests use `/graphql`.
- Asset uploads use `/u`.
- API authentication uses `Authorization: Bearer <token>`.

## Development

```bash
go test ./...
go vet ./...
GOOS=linux GOARCH=amd64 go build -o /tmp/wikijs-linux-amd64 ./cmd/wikijs
GOOS=darwin GOARCH=arm64 go build -o /tmp/wikijs-darwin-arm64 ./cmd/wikijs
GOOS=windows GOARCH=amd64 go build -o /tmp/wikijs-windows-amd64.exe ./cmd/wikijs
```

## License

GPLv3
