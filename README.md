# wikijs-cli

A Go command-line client for Wiki.js 2.x.

Forked from [hopyky/wikijs-cli](https://github.com/hopyky/wikijs-cli) (no longer available) which was written in JavaScript. The majority of
features have been ported over, with some minor changes to naming and additional support.

## Features

- Full CRUD operations - Create, read, update, and delete wiki pages
- Tag management - Add, remove, set, and list tags
- Asset management - Upload, list, and delete files
- Search - Full-text page search and content grep
- Backup & Restore - Export and import wiki content
- Bulk operations - Create/update multiple pages from files with progress bars
- Version control - View history and revert to previous versions
- Link checker - Find broken internal links
- Page validation - Check images, links, and content quality
- Templates - Create pages from reusable templates
- Shell completion - Auto-completion for bash/zsh/fish
- Tree view - Visual hierarchy of all pages
- Interactive shell - REPL mode for multiple commands
- Markdown linting - Validate markdown issues
- Search & replace - Find and replace text across pages
- Local sync - Download wiki pages to a local directory
- Rate limiting - Configurable delays for bulk operations and sync

## Install

```bash
go install github.com/dfuentes87/wikijs-cli/cmd/wikijs@latest
```

## Configuration

Copy the example config into your home directory and edit it. Default paths:

- macOS/Linux: `~/.config/wikijs.json`
- Windows: `%AppData%\wikijs\config.json`

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `url` | Wiki.js server URL | Required |
| `apiToken` | API authentication token | Required |
| `defaultEditor` | Default editor type | `markdown` |
| `defaultLocale` | Default page locale | `en` |
| `autoSync.enabled` | Reserved for future automatic sync | `false` |
| `autoSync.path` | Default `sync` output directory | - |
| `autoSync.intervalHours` | Reserved sync interval | `24` |
| `backup.enabled` | Enable backups | `true` |
| `backup.path` | Backup directory | - |
| `backup.keepDays` | Backup retention | `30` |

### Environment Variables

You can override the config path with `--config` or `WIKIJS_CONFIG`.
If `WIKIJS_URL` and `WIKIJS_API_TOKEN` are set, the CLI can run without a
config file.

Environment variables override values:

- `WIKIJS_URL`
- `WIKIJS_API_TOKEN`
- `WIKIJS_DEFAULT_LOCALE`
- `WIKIJS_DEFAULT_EDITOR`

### Getting an API Token

1. Log in to your Wiki.js instance as an administrator
2. Go to Administration > API Access
3. Create a new API key with the required permissions
4. Copy the token to your configuration file

## Usage

### Basic Commands

```bash
# Check connection/version
wikijs version
wikijs health

# Interactive shell mode
wikijs shell

# List pages
wikijs list
wikijs list --limit 20 --locale en --tag docs

# Search pages
wikijs search "configuration"

# Get a page
wikijs get 1                    # by ID
wikijs get /docs/guide --raw    # by path
wikijs get 1 --raw --metadata   # with metadata header
```

### Creating & Editing Pages

```bash
# Create a page with inline content
wikijs create /docs/intro "Introduction" --content "# Welcome"
# Create from a file
wikijs create /docs/guide "Guide" --file ./guide.md --tag docs,guide
# Create from stdin
wikijs create /docs/readme "README" --stdin

# Update a page
wikijs update 1 --content "Updated content"
wikijs update 1 --file ./updated.md
wikijs update 1 --title "New title" --tags docs,important

# Move a page
wikijs move 1 /new/path

# Delete a page
wikijs delete 1
wikijs delete 1 --force # Skip confirmation
```

### Tag Management

```bash
# List all tags
wikijs tags
wikijs tags --format json

# Manage page tags
wikijs tag 1 add important
wikijs tag 1 remove draft
wikijs tag 1 set docs,guide
```

### Search & Discovery

```bash
# Full-text search
wikijs search "configuration"

# Search within page content
wikijs grep "TODO"
wikijs grep "deprecated" --path /docs --case-sensitive

# Get page info
wikijs info 1
wikijs info /docs/guide

# View statistics
wikijs stats
wikijs stats --detailed
```

### Backup & Restore

```bash
# Create a backup
wikijs backup --output backup.json
wikijs backup --output - --format json

# Export pages to files
wikijs export ./pages
wikijs export ./pages-json --file-format json --format json

# Restore from backup
wikijs restore-backup backup.json --dry-run
wikijs restore-backup backup.json --skip-existing
wikijs restore-backup backup.json --force
```

### Version Control

```bash
# View page history
wikijs versions 1

# Revert to a previous version
wikijs revert 1 5
```

### Bulk Operations

```bash
# Create pages from a folder of markdown files
wikijs bulk-create ./pages --path-prefix /docs --tag imported
wikijs bulk-create ./pages --dry-run
# Update existing pages from files
wikijs bulk-update ./pages --path-prefix /docs
wikijs bulk-update ./pages --skip-missing

# Sync all pages to local directory
wikijs sync --output ./local-wiki
wikijs sync --output ./local-wiki --file-format json --format json
wikijs sync --path /docs --delete
```

### Asset Management

```bash
# List assets
wikijs asset list
wikijs asset list --folder /uploads --limit 100

# Upload a file
wikijs asset upload ./image.png
wikijs asset upload ./image.png --rename hero.png

# Delete an asset
wikijs asset delete 42
```

### Tree View & Analysis

```bash
# Display page hierarchy as a tree
wikijs tree
wikijs tree --locale en

# Find broken internal links
wikijs check-links
wikijs check-links --path /docs

# Compare page versions
wikijs diff 1
wikijs diff 1 5
wikijs diff 1 4 5
```

### Page Operations

```bash
# Clone/duplicate a page
wikijs clone 1 /docs/copy-of-page
wikijs clone /docs/source /docs/copy --with-tags

# Search and replace across pages
wikijs replace "old term" "new term" --dry-run
wikijs replace "old term" "new term" --path /docs --force
wikijs replace "old[0-9]+" "new" --regex --case-sensitive --force
```

### Content Quality

```bash
# Lint markdown files
wikijs lint ./document.md
wikijs lint ./document.md --format json

# Validate page content (images, links, quality)
wikijs validate 1
wikijs validate --all --format json
```

### Templates

```bash
# List available templates
wikijs template list

# Create a template
wikijs template create doc --content "# {{title}}\n\nCreated: {{date}}"
wikijs template create doc --file ./template.md

# Use a template when creating a page
wikijs create /docs/new "New Page" --template doc

# Show/delete templates
wikijs template show doc
wikijs template delete doc --force
```

### Shell Completion

Completion scripts are provided by Cobra:

```bash
wikijs completion bash
wikijs completion zsh
wikijs completion fish
wikijs completion powershell
```

## Global Flags

```bash
wikijs --config ./wikijs.json <command>
wikijs --format json <command>
wikijs --verbose <command>
wikijs --debug <command>
wikijs --rate-limit 500 <command>
```

Supported output formats are `table` and `json`.
`--verbose` logs request paths and HTTP statuses to stderr. `--debug` also logs.

Use `wikijs completion --help` for shell-specific installation instructions.

## Troubleshooting

### Connection Refused

Verify the url in your config file is correct
Ensure Wiki.js is running and accessible
Check firewall settings

### Authentication Failed

Regenerate your API token in Wiki.js admin
Ensure the token has the required permissions
Check the token hasn't expired

### Page Not Found (by path)

Paths are case-sensitive
Use wikijs list to see existing paths
Try specifying --locale if using multiple languages

## API Compatibility

The CLI targets Wiki.js 2.x:

- GraphQL requests use `/graphql`.
- Asset uploads use `/u`.
- API authentication uses `Authorization: Bearer <token>`.

## Contributing

See [CONTRIBUTING.md](https://github.com/dfuentes87/wikijs-cli/CONTRIBUTING.md)

## Development

```bash
git clone https://github.com/dfuentes87/wikijs-cli.git
go build ./cmd/wikijs
go test ./...
go vet ./...
GOOS=linux GOARCH=amd64 go build -o /tmp/wikijs-linux-amd64 ./cmd/wikijs
GOOS=darwin GOARCH=arm64 go build -o /tmp/wikijs-darwin-arm64 ./cmd/wikijs
GOOS=windows GOARCH=amd64 go build -o /tmp/wikijs-windows-amd64.exe ./cmd/wikijs
```

## License

GPLv3
