package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dfuentes87/wikijs-cli/internal/api"
	"github.com/dfuentes87/wikijs-cli/internal/config"
	"github.com/dfuentes87/wikijs-cli/internal/markdown"
	"github.com/dfuentes87/wikijs-cli/internal/output"
	"github.com/dfuentes87/wikijs-cli/internal/tree"
)

type WikiClient interface {
	ListPages(context.Context, api.ListOptions) ([]api.Page, error)
	SearchPages(context.Context, string, int) (api.SearchResult, error)
	GetPage(context.Context, string, string, bool) (api.Page, error)
	CreatePage(context.Context, api.CreatePageInput) (api.Page, error)
	UpdatePage(context.Context, api.UpdatePageInput) (api.Page, error)
	MovePage(context.Context, int, string, string) error
	DeletePage(context.Context, int) error
	ListTags(context.Context) ([]api.Tag, error)
	ListAssets(context.Context, string, int) ([]api.Asset, error)
	UploadAsset(context.Context, string, string) (map[string]any, error)
	DeleteAsset(context.Context, int) error
	Health(context.Context) (api.SystemInfo, error)
	Stats(context.Context) (api.Stats, error)
	PageVersions(context.Context, int) ([]api.Version, error)
	GetPageVersion(context.Context, int, int) (api.PageVersion, error)
	RevertPage(context.Context, int, int) error
}

type app struct {
	configPath string
	format     string
	verbose    bool
	debug      bool
	noColor    bool
	rateLimit  int
	out        io.Writer
	errOut     io.Writer
	in         io.Reader
	client     WikiClient
}

func NewRootCommand() *cobra.Command {
	a := &app{format: "table", out: os.Stdout, errOut: os.Stderr, in: os.Stdin}
	return newRootCommand(a)
}

func newRootCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "wikijs",
		Short:         "CLI for Wiki.js",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(a.out)
	cmd.SetErr(a.errOut)
	cmd.SetIn(a.in)
	cmd.PersistentFlags().StringVar(&a.configPath, "config", "", "config file path")
	cmd.PersistentFlags().StringVarP(&a.format, "format", "f", "table", "output format: table or json")
	cmd.PersistentFlags().BoolVar(&a.verbose, "verbose", false, "enable verbose output")
	cmd.PersistentFlags().BoolVar(&a.debug, "debug", false, "enable debug output")
	cmd.PersistentFlags().BoolVar(&a.noColor, "no-color", false, "disable ANSI color output")
	cmd.PersistentFlags().IntVar(&a.rateLimit, "rate-limit", 0, "delay between API requests in milliseconds")

	cmd.AddCommand(
		a.healthCommand(),
		a.listCommand(),
		a.searchCommand(),
		a.getCommand(),
		a.createCommand(),
		a.updateCommand(),
		a.moveCommand(),
		a.deleteCommand(),
		a.tagsCommand(),
		a.tagCommand(),
		a.statsCommand(),
		a.infoCommand(),
		a.grepCommand(),
		a.versionsCommand(),
		a.revertCommand(),
		a.assetCommand(),
		a.treeCommand(),
		a.lintCommand(),
		a.backupCommand(),
		a.restoreBackupCommand(),
		a.exportCommand(),
		a.syncCommand(),
		a.checkLinksCommand(),
		a.diffCommand(),
		a.cloneCommand(),
		a.validateCommand(),
		a.bulkCreateCommand(),
		a.bulkUpdateCommand(),
		a.bulkMoveCommand(),
		a.templateCommand(),
		a.replaceCommand(),
		a.shellCommand(),
	)
	return cmd
}

func CommandColorEnabled(cmd *cobra.Command) bool {
	if cmd == nil {
		return true
	}
	root := cmd.Root()
	if root == nil {
		root = cmd
	}
	noColor := false
	if flag := root.PersistentFlags().Lookup("no-color"); flag != nil {
		noColor = flag.Value.String() == "true"
	}
	format := "table"
	if flag := root.PersistentFlags().Lookup("format"); flag != nil {
		format = flag.Value.String()
	}
	return !noColor && format == "table"
}

func (a *app) getClient() (WikiClient, error) {
	if a.client != nil {
		return a.client, nil
	}
	cfg, path, err := config.Load(a.configPath)
	if err != nil {
		if errors.Is(err, config.ErrMissing) {
			return nil, fmt.Errorf("%w; copy config/wikijs.example.json to %s and configure it", err, path)
		}
		return nil, err
	}
	if a.debug || a.verbose {
		fmt.Fprintf(a.errOut, "Config: %s\n", path)
	}
	opts := []api.Option{api.WithRateLimit(time.Duration(a.rateLimit) * time.Millisecond)}
	if a.debug || a.verbose {
		opts = append(opts, api.WithLogger(func(format string, args ...any) {
			fmt.Fprintf(a.errOut, format+"\n", args...)
		}, a.debug))
	}
	a.client = api.New(cfg, opts...)
	return a.client, nil
}

func (a *app) print(data any, headers []string, rows [][]string) error {
	if a.format == "json" {
		return output.JSON(a.out, data)
	}
	if a.format != "table" {
		return fmt.Errorf("unsupported format %q", a.format)
	}
	return output.Table(a.out, headers, rows)
}

func (a *app) colorEnabled() bool {
	return !a.noColor && a.format == "table"
}

func (a *app) color(code, text string) string {
	return output.Color(a.colorEnabled(), code, text)
}

func (a *app) success(text string) string {
	return a.color(output.Green, text)
}

func (a *app) warn(text string) string {
	return a.color(output.Yellow, text)
}

type successResult struct {
	Success   bool   `json:"success"`
	Action    string `json:"action"`
	ID        int    `json:"id,omitempty"`
	Path      string `json:"path,omitempty"`
	PageID    int    `json:"pageId,omitempty"`
	VersionID int    `json:"versionId,omitempty"`
	Message   string `json:"message,omitempty"`
	Result    any    `json:"result,omitempty"`
}

func (a *app) healthCommand() *cobra.Command {
	return &cobra.Command{Use: "health", Short: "Check Wiki.js connection", RunE: func(cmd *cobra.Command, args []string) error {
		client, err := a.getClient()
		if err != nil {
			return err
		}
		info, err := client.Health(cmd.Context())
		if err != nil {
			return err
		}
		return a.print(info, []string{"Version", "Latest", "Host", "Platform"}, [][]string{{info.CurrentVersion, info.LatestVersion, info.Hostname, info.Platform}})
	}}
}

func (a *app) listCommand() *cobra.Command {
	var opts api.ListOptions
	cmd := &cobra.Command{Use: "list", Short: "List pages", RunE: func(cmd *cobra.Command, args []string) error {
		client, err := a.getClient()
		if err != nil {
			return err
		}
		pages, err := client.ListPages(cmd.Context(), opts)
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(pages))
		for _, page := range pages {
			rows = append(rows, []string{strconv.Itoa(page.ID), page.Path, page.Title, page.Locale, output.Bool(page.IsPublished), output.Date(page.UpdatedAt)})
		}
		return a.print(pages, []string{"ID", "Path", "Title", "Locale", "Published", "Updated"}, rows)
	}}
	cmd.Flags().StringVar(&opts.Tag, "tag", "", "filter by tag")
	cmd.Flags().StringVar(&opts.Locale, "locale", "", "filter by locale")
	cmd.Flags().IntVar(&opts.Limit, "limit", 50, "maximum results")
	return cmd
}

func (a *app) searchCommand() *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "search <query>", Short: "Search pages", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client, err := a.getClient()
		if err != nil {
			return err
		}
		result, err := client.SearchPages(cmd.Context(), args[0], limit)
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(result.Results))
		for _, page := range result.Results {
			rows = append(rows, []string{strconv.Itoa(page.ID), page.Path, page.Title, page.Locale})
		}
		return a.print(result, []string{"ID", "Path", "Title", "Locale"}, rows)
	}}
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum results")
	return cmd
}

func (a *app) getCommand() *cobra.Command {
	var raw, metadata, children bool
	var locale string
	cmd := &cobra.Command{Use: "get <id-or-path>", Short: "Get a page", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client, err := a.getClient()
		if err != nil {
			return err
		}
		page, err := client.GetPage(cmd.Context(), args[0], locale, children)
		if err != nil {
			return err
		}
		if raw {
			if metadata {
				fmt.Fprintf(a.out, "---\nid: %d\npath: %s\ntitle: %s\nlocale: %s\n---\n", page.ID, page.Path, page.Title, page.Locale)
			}
			_, err := fmt.Fprint(a.out, page.Content)
			return err
		}
		rows := [][]string{{"ID", strconv.Itoa(page.ID)}, {"Path", page.Path}, {"Title", page.Title}, {"Locale", page.Locale}, {"Published", output.Bool(page.IsPublished)}, {"Updated", output.Date(page.UpdatedAt)}}
		return a.print(page, []string{"Field", "Value"}, rows)
	}}
	cmd.Flags().BoolVar(&raw, "raw", false, "print raw page content")
	cmd.Flags().BoolVar(&metadata, "metadata", false, "include metadata header with --raw")
	cmd.Flags().BoolVar(&children, "children", false, "include child pages")
	cmd.Flags().StringVar(&locale, "locale", "", "page locale for path lookups")
	return cmd
}

func (a *app) createCommand() *cobra.Command {
	var file, content, tags, locale, editor, description, templateName string
	var stdin, draft, private bool
	cmd := &cobra.Command{Use: "create <path> <title>", Short: "Create a page", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		body, err := readContent(a.in, file, content, stdin)
		if err != nil {
			return err
		}
		if templateName != "" {
			rendered, err := a.renderTemplate(templateName, map[string]string{
				"title": args[1],
				"path":  strings.TrimPrefix(args[0], "/"),
				"date":  time.Now().Format("2006-01-02"),
			})
			if err != nil {
				return err
			}
			if body == "" {
				body = rendered
			}
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		page, err := client.CreatePage(cmd.Context(), api.CreatePageInput{Path: args[0], Title: args[1], Content: body, Description: description, Tags: parseTags(tags), Locale: locale, Editor: editor, IsPublished: !draft, IsPrivate: private})
		if err != nil {
			return err
		}
		return a.print(page, []string{"ID", "Path", "Title"}, [][]string{{strconv.Itoa(page.ID), page.Path, page.Title}})
	}}
	cmd.Flags().StringVar(&file, "file", "", "read content from file")
	cmd.Flags().StringVar(&content, "content", "", "inline content")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read content from stdin")
	cmd.Flags().StringVar(&tags, "tag", "", "comma-separated tags")
	cmd.Flags().StringVar(&templateName, "template", "", "template name")
	cmd.Flags().StringVar(&locale, "locale", "", "page locale")
	cmd.Flags().StringVar(&editor, "editor", "", "editor type")
	cmd.Flags().StringVar(&description, "description", "", "page description")
	cmd.Flags().BoolVar(&draft, "draft", false, "create as unpublished")
	cmd.Flags().BoolVar(&private, "private", false, "create as private")
	return cmd
}

func (a *app) updateCommand() *cobra.Command {
	var file, content, title, description, tags string
	var stdin, published, unpublished bool
	cmd := &cobra.Command{Use: "update <id>", Short: "Update a page", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}
		body, hasBody, err := readOptionalContent(a.in, file, content, stdin)
		if err != nil {
			return err
		}
		input := api.UpdatePageInput{ID: id}
		if hasBody {
			input.Content = &body
		}
		if title != "" {
			input.Title = &title
		}
		if description != "" {
			input.Description = &description
		}
		if tags != "" {
			input.SetTags = true
			input.Tags = parseTags(tags)
		}
		if published && unpublished {
			return errors.New("--published and --unpublished cannot both be set")
		}
		if published || unpublished {
			value := published
			input.IsPublished = &value
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		page, err := client.UpdatePage(cmd.Context(), input)
		if err != nil {
			return err
		}
		return a.print(page, []string{"ID", "Path", "Title", "Updated"}, [][]string{{strconv.Itoa(page.ID), page.Path, page.Title, output.Date(page.UpdatedAt)}})
	}}
	cmd.Flags().StringVar(&file, "file", "", "read content from file")
	cmd.Flags().StringVar(&content, "content", "", "inline content")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read content from stdin")
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().StringVar(&tags, "tags", "", "replace tags with comma-separated list")
	cmd.Flags().BoolVar(&published, "published", false, "mark as published")
	cmd.Flags().BoolVar(&unpublished, "unpublished", false, "mark as unpublished")
	return cmd
}

func (a *app) moveCommand() *cobra.Command {
	var locale string
	cmd := &cobra.Command{Use: "move <id> <new-path>", Short: "Move a page", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		if err := client.MovePage(cmd.Context(), id, args[1], locale); err != nil {
			return err
		}
		if a.format == "json" {
			return output.JSON(a.out, successResult{Success: true, Action: "move", ID: id, Path: args[1]})
		}
		_, err = fmt.Fprintf(a.out, "%s\n", a.success(fmt.Sprintf("Moved page %d to %s", id, args[1])))
		return err
	}}
	cmd.Flags().StringVar(&locale, "locale", "", "destination locale")
	return cmd
}

func (a *app) deleteCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{Use: "delete <id>", Short: "Delete a page", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}
		if !force && !a.confirm(fmt.Sprintf("Delete page %d? This cannot be undone.", id)) {
			return errors.New("delete cancelled")
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		if err := client.DeletePage(cmd.Context(), id); err != nil {
			return err
		}
		if a.format == "json" {
			return output.JSON(a.out, successResult{Success: true, Action: "delete", ID: id})
		}
		_, err = fmt.Fprintf(a.out, "%s\n", a.success(fmt.Sprintf("Deleted page %d", id)))
		return err
	}}
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")
	return cmd
}

func (a *app) tagsCommand() *cobra.Command {
	return &cobra.Command{Use: "tags", Short: "List tags", RunE: func(cmd *cobra.Command, args []string) error {
		client, err := a.getClient()
		if err != nil {
			return err
		}
		tags, err := client.ListTags(cmd.Context())
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(tags))
		for _, tag := range tags {
			rows = append(rows, []string{strconv.Itoa(tag.ID), tag.Tag, tag.Title})
		}
		return a.print(tags, []string{"ID", "Tag", "Title"}, rows)
	}}
}

func (a *app) versionsCommand() *cobra.Command {
	return &cobra.Command{Use: "versions <id>", Short: "List page versions", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		versions, err := client.PageVersions(cmd.Context(), id)
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(versions))
		for _, version := range versions {
			rows = append(rows, []string{strconv.Itoa(version.VersionID), output.Date(version.VersionDate), version.AuthorName, version.ActionType})
		}
		return a.print(versions, []string{"Version", "Date", "Author", "Action"}, rows)
	}}
}

func (a *app) revertCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{Use: "revert <page-id> <version-id>", Short: "Revert page to a version", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		pageID, err := parseID(args[0])
		if err != nil {
			return err
		}
		versionID, err := parseID(args[1])
		if err != nil {
			return err
		}
		if !force && !a.confirm(fmt.Sprintf("Revert page %d to version %d? This changes live content.", pageID, versionID)) {
			return errors.New("revert cancelled")
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		if err := client.RevertPage(cmd.Context(), pageID, versionID); err != nil {
			return err
		}
		if a.format == "json" {
			return output.JSON(a.out, successResult{Success: true, Action: "revert", PageID: pageID, VersionID: versionID})
		}
		_, err = fmt.Fprintf(a.out, "%s\n", a.success(fmt.Sprintf("Reverted page %d to version %d", pageID, versionID)))
		return err
	}}
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")
	return cmd
}

func (a *app) assetCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "asset", Short: "Manage assets"}
	var folder string
	var limit int
	list := &cobra.Command{Use: "list", Short: "List assets", RunE: func(cmd *cobra.Command, args []string) error {
		client, err := a.getClient()
		if err != nil {
			return err
		}
		assets, err := client.ListAssets(cmd.Context(), folder, limit)
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(assets))
		for _, asset := range assets {
			rows = append(rows, []string{strconv.Itoa(asset.ID), asset.Filename, asset.Kind, output.Bytes(asset.FileSize)})
		}
		return a.print(assets, []string{"ID", "Filename", "Kind", "Size"}, rows)
	}}
	list.Flags().StringVar(&folder, "folder", "", "filter by folder path")
	list.Flags().IntVar(&limit, "limit", 50, "maximum results")
	var rename string
	upload := &cobra.Command{Use: "upload <file>", Short: "Upload an asset", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client, err := a.getClient()
		if err != nil {
			return err
		}
		result, err := client.UploadAsset(cmd.Context(), args[0], rename)
		if err != nil {
			return err
		}
		if a.format == "json" {
			return output.JSON(a.out, successResult{Success: true, Action: "upload", Message: "Asset uploaded", Result: result})
		}
		_, err = fmt.Fprintln(a.out, a.success("Asset uploaded"))
		return err
	}}
	upload.Flags().StringVar(&rename, "rename", "", "upload with a different filename")
	var force bool
	del := &cobra.Command{Use: "delete <id>", Short: "Delete an asset", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}
		if !force && !a.confirm(fmt.Sprintf("Delete asset %d? This cannot be undone.", id)) {
			return errors.New("delete cancelled")
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		if err := client.DeleteAsset(cmd.Context(), id); err != nil {
			return err
		}
		if a.format == "json" {
			return output.JSON(a.out, successResult{Success: true, Action: "delete_asset", ID: id})
		}
		_, err = fmt.Fprintf(a.out, "%s\n", a.success(fmt.Sprintf("Deleted asset %d", id)))
		return err
	}}
	del.Flags().BoolVar(&force, "force", false, "skip confirmation")
	cmd.AddCommand(list, upload, del)
	return cmd
}

func (a *app) treeCommand() *cobra.Command {
	var locale string
	cmd := &cobra.Command{Use: "tree", Short: "Show page tree", RunE: func(cmd *cobra.Command, args []string) error {
		client, err := a.getClient()
		if err != nil {
			return err
		}
		pages, err := client.ListPages(cmd.Context(), api.ListOptions{Locale: locale, Limit: 0})
		if err != nil {
			return err
		}
		if a.format == "json" {
			return output.JSON(a.out, pages)
		}
		_, err = fmt.Fprintln(a.out, tree.Render(pages))
		return err
	}}
	cmd.Flags().StringVar(&locale, "locale", "", "filter by locale")
	return cmd
}

func (a *app) lintCommand() *cobra.Command {
	return &cobra.Command{Use: "lint <file>", Short: "Lint a Markdown file", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		result := markdown.Lint(string(data))
		if a.format == "json" {
			return output.JSON(a.out, result)
		}
		_, err = fmt.Fprintln(a.out, markdown.Format(result))
		if err != nil {
			return err
		}
		if !result.Valid {
			return errors.New("markdown lint failed")
		}
		return nil
	}}
}

func (a *app) confirm(prompt string) bool {
	fmt.Fprintf(a.errOut, "%s Type 'yes' to continue: ", prompt)
	scanner := bufio.NewScanner(a.in)
	if !scanner.Scan() {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(scanner.Text()), "yes")
}

func readContent(in io.Reader, file, inline string, useStdin bool) (string, error) {
	content, hasContent, err := readOptionalContent(in, file, inline, useStdin)
	if err != nil {
		return "", err
	}
	if !hasContent {
		return "", nil
	}
	return content, nil
}

func readOptionalContent(in io.Reader, file, inline string, useStdin bool) (string, bool, error) {
	count := 0
	if file != "" {
		count++
	}
	if inline != "" {
		count++
	}
	if useStdin {
		count++
	}
	if count > 1 {
		return "", false, errors.New("use only one of --file, --content, or --stdin")
	}
	switch {
	case file != "":
		data, err := os.ReadFile(file)
		return string(data), true, err
	case inline != "":
		return inline, true, nil
	case useStdin:
		data, err := io.ReadAll(in)
		return string(data), true, err
	default:
		return "", false, nil
	}
}

func parseID(value string) (int, error) {
	id, err := strconv.Atoi(value)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid id: %s", value)
	}
	return id, nil
}

func parseTags(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}
