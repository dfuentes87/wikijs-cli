package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dfuentes87/wikijs-cli/internal/api"
	"github.com/dfuentes87/wikijs-cli/internal/output"
)

type backupFile struct {
	Version    int        `json:"version"`
	ExportedAt string     `json:"exportedAt"`
	Pages      []api.Page `json:"pages"`
}

type operationSummary struct {
	Created int      `json:"created,omitempty"`
	Updated int      `json:"updated,omitempty"`
	Skipped int      `json:"skipped,omitempty"`
	Matched int      `json:"matched,omitempty"`
	Changed int      `json:"changed,omitempty"`
	Files   int      `json:"files,omitempty"`
	Pages   int      `json:"pages,omitempty"`
	Paths   []string `json:"paths,omitempty"`
}

func (a *app) backupCommand() *cobra.Command {
	var outputPath string
	cmd := &cobra.Command{Use: "backup", Short: "Export wiki content to a JSON backup", RunE: func(cmd *cobra.Command, args []string) error {
		client, err := a.getClient()
		if err != nil {
			return err
		}
		pages, err := client.ListPages(cmd.Context(), api.ListOptions{Limit: 0})
		if err != nil {
			return err
		}
		fullPages := make([]api.Page, 0, len(pages))
		for i, page := range pages {
			a.progress("Backing up", i+1, len(pages))
			fullPage, err := client.GetPage(cmd.Context(), strconvItoa(page.ID), "", false)
			if err != nil {
				return err
			}
			fullPages = append(fullPages, fullPage)
		}
		a.progressDone()
		backup := backupFile{Version: 1, ExportedAt: time.Now().UTC().Format(time.RFC3339), Pages: fullPages}
		if outputPath == "" || outputPath == "-" {
			return output.JSON(a.out, backup)
		}
		if err := writeJSONFile(outputPath, backup); err != nil {
			return err
		}
		if a.format == "json" {
			return output.JSON(a.out, successResult{Success: true, Action: "backup", Path: outputPath, Result: operationSummary{Pages: len(fullPages)}})
		}
		_, err = fmt.Fprintf(a.out, "Backed up %d pages to %s\n", len(fullPages), outputPath)
		return err
	}}
	cmd.Flags().StringVarP(&outputPath, "output", "o", "wikijs-backup.json", "backup output path, or - for stdout")
	return cmd
}

func (a *app) restoreBackupCommand() *cobra.Command {
	var dryRun, skipExisting, force bool
	cmd := &cobra.Command{Use: "restore-backup <file>", Short: "Import wiki content from a JSON backup", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		var backup backupFile
		if err := readJSONFile(args[0], &backup); err != nil {
			return err
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		summary := operationSummary{}
		for i, page := range backup.Pages {
			a.progress("Restoring", i+1, len(backup.Pages))
			existing, getErr := client.GetPage(cmd.Context(), page.Path, page.Locale, false)
			if getErr == nil {
				if skipExisting {
					summary.Skipped++
					continue
				}
				if !force && !dryRun {
					return fmt.Errorf("page already exists at %s; use --force to overwrite or --skip-existing", page.Path)
				}
				if !dryRun {
					content := page.Content
					title := page.Title
					description := page.Description
					tags := []string(page.Tags)
					_, err := client.UpdatePage(cmd.Context(), api.UpdatePageInput{
						ID: existing.ID, Content: &content, Title: &title, Description: &description, Tags: tags, SetTags: true,
					})
					if err != nil {
						return err
					}
				}
				summary.Updated++
				continue
			}
			if !errors.Is(getErr, api.ErrNotFound) {
				return getErr
			}
			if !dryRun {
				_, err := client.CreatePage(cmd.Context(), api.CreatePageInput{
					Path: page.Path, Title: page.Title, Content: page.Content, Description: page.Description,
					Tags: []string(page.Tags), Locale: page.Locale, IsPublished: page.IsPublished, IsPrivate: page.IsPrivate,
				})
				if err != nil {
					return err
				}
			}
			summary.Created++
		}
		a.progressDone()
		if a.format == "json" {
			return output.JSON(a.out, successResult{Success: true, Action: "restore-backup", Result: summary})
		}
		_, err = fmt.Fprintf(a.out, "Restore complete: %d created, %d updated, %d skipped\n", summary.Created, summary.Updated, summary.Skipped)
		return err
	}}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be restored without changing Wiki.js")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "skip pages that already exist")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing pages")
	return cmd
}

func (a *app) bulkCreateCommand() *cobra.Command {
	var prefix, tags, locale, editor string
	var dryRun bool
	cmd := &cobra.Command{Use: "bulk-create <directory>", Short: "Create pages from Markdown files", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		files, err := markdownFiles(args[0])
		if err != nil {
			return err
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		summary := operationSummary{Files: len(files)}
		for i, file := range files {
			a.progress("Creating", i+1, len(files))
			content, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			path, err := pagePathFromFile(args[0], file, prefix)
			if err != nil {
				return err
			}
			title := titleFromMarkdown(string(content), path)
			if dryRun {
				summary.Created++
				summary.Paths = append(summary.Paths, path)
				continue
			}
			_, err = client.CreatePage(cmd.Context(), api.CreatePageInput{
				Path: path, Title: title, Content: string(content), Tags: parseTags(tags), Locale: locale, Editor: editor, IsPublished: true,
			})
			if err != nil {
				return err
			}
			summary.Created++
			summary.Paths = append(summary.Paths, path)
		}
		a.progressDone()
		return a.printSummary("bulk-create", summary)
	}}
	cmd.Flags().StringVar(&prefix, "path-prefix", "", "wiki path prefix for created pages")
	cmd.Flags().StringVar(&tags, "tag", "", "comma-separated tags")
	cmd.Flags().StringVar(&locale, "locale", "", "page locale")
	cmd.Flags().StringVar(&editor, "editor", "", "editor type")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show pages without creating them")
	return cmd
}

func (a *app) bulkUpdateCommand() *cobra.Command {
	var prefix string
	var dryRun, skipMissing bool
	cmd := &cobra.Command{Use: "bulk-update <directory>", Short: "Update pages from Markdown files", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		files, err := markdownFiles(args[0])
		if err != nil {
			return err
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		summary := operationSummary{Files: len(files)}
		for i, file := range files {
			a.progress("Updating", i+1, len(files))
			contentBytes, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			path, err := pagePathFromFile(args[0], file, prefix)
			if err != nil {
				return err
			}
			page, err := client.GetPage(cmd.Context(), path, "", false)
			if err != nil {
				if skipMissing && errors.Is(err, api.ErrNotFound) {
					summary.Skipped++
					continue
				}
				return err
			}
			if !dryRun {
				content := string(contentBytes)
				_, err = client.UpdatePage(cmd.Context(), api.UpdatePageInput{ID: page.ID, Content: &content})
				if err != nil {
					return err
				}
			}
			summary.Updated++
			summary.Paths = append(summary.Paths, path)
		}
		a.progressDone()
		return a.printSummary("bulk-update", summary)
	}}
	cmd.Flags().StringVar(&prefix, "path-prefix", "", "wiki path prefix for updated pages")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show pages without updating them")
	cmd.Flags().BoolVar(&skipMissing, "skip-missing", false, "skip files whose pages do not exist")
	return cmd
}

func (a *app) templateCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "template", Short: "Manage reusable page templates"}
	cmd.AddCommand(a.templateListCommand(), a.templateCreateCommand(), a.templateShowCommand(), a.templateDeleteCommand())
	return cmd
}

func (a *app) templateListCommand() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List templates", RunE: func(cmd *cobra.Command, args []string) error {
		names, err := listTemplates()
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(names))
		for _, name := range names {
			rows = append(rows, []string{name})
		}
		return a.print(names, []string{"Name"}, rows)
	}}
}

func (a *app) templateCreateCommand() *cobra.Command {
	var file, content string
	var stdin bool
	cmd := &cobra.Command{Use: "create <name>", Short: "Create or replace a template", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		body, err := readContent(a.in, file, content, stdin)
		if err != nil {
			return err
		}
		if body == "" {
			return errors.New("template content is required")
		}
		if err := saveTemplate(args[0], body); err != nil {
			return err
		}
		if a.format == "json" {
			return output.JSON(a.out, successResult{Success: true, Action: "template-create", Path: args[0]})
		}
		_, err = fmt.Fprintf(a.out, "Saved template %s\n", args[0])
		return err
	}}
	cmd.Flags().StringVar(&file, "file", "", "read template from file")
	cmd.Flags().StringVar(&content, "content", "", "inline template content")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read template from stdin")
	return cmd
}

func (a *app) templateShowCommand() *cobra.Command {
	return &cobra.Command{Use: "show <name>", Short: "Show a template", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		content, err := loadTemplate(args[0])
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(a.out, content)
		return err
	}}
}

func (a *app) templateDeleteCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{Use: "delete <name>", Short: "Delete a template", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !force && !a.confirm(fmt.Sprintf("Delete template %s?", args[0])) {
			return errors.New("delete cancelled")
		}
		if err := deleteTemplate(args[0]); err != nil {
			return err
		}
		if a.format == "json" {
			return output.JSON(a.out, successResult{Success: true, Action: "template-delete", Path: args[0]})
		}
		_, err := fmt.Fprintf(a.out, "Deleted template %s\n", args[0])
		return err
	}}
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")
	return cmd
}

func (a *app) replaceCommand() *cobra.Command {
	var pathPrefix string
	var dryRun, regexMode, caseSensitive, force bool
	cmd := &cobra.Command{Use: "replace <old> <new>", Short: "Find and replace text across pages", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if !dryRun && !force && !a.confirm("Replace content across matching pages? This changes live content.") {
			return errors.New("replace cancelled")
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		pages, err := client.ListPages(cmd.Context(), api.ListOptions{Limit: 0})
		if err != nil {
			return err
		}
		replacer, err := newReplacer(args[0], args[1], regexMode, caseSensitive)
		if err != nil {
			return err
		}
		summary := operationSummary{}
		for i, listed := range pages {
			a.progress("Replacing", i+1, len(pages))
			if pathPrefix != "" && !strings.HasPrefix(strings.Trim(listed.Path, "/"), strings.Trim(pathPrefix, "/")) {
				continue
			}
			page, err := client.GetPage(cmd.Context(), strconvItoa(listed.ID), "", false)
			if err != nil {
				return err
			}
			newContent, changed := replacer(page.Content)
			if !changed {
				continue
			}
			summary.Matched++
			summary.Paths = append(summary.Paths, page.Path)
			if !dryRun {
				_, err = client.UpdatePage(cmd.Context(), api.UpdatePageInput{ID: page.ID, Content: &newContent})
				if err != nil {
					return err
				}
				summary.Changed++
			}
		}
		a.progressDone()
		return a.printSummary("replace", summary)
	}}
	cmd.Flags().StringVar(&pathPrefix, "path", "", "only replace pages under this path")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show matches without updating pages")
	cmd.Flags().BoolVar(&regexMode, "regex", false, "treat old text as a regular expression")
	cmd.Flags().BoolVar(&caseSensitive, "case-sensitive", false, "use case-sensitive matching")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")
	return cmd
}

func (a *app) shellCommand() *cobra.Command {
	return &cobra.Command{Use: "shell", Short: "Run an interactive wikijs shell", RunE: func(cmd *cobra.Command, args []string) error {
		scanner := bufio.NewScanner(a.in)
		for {
			fmt.Fprint(a.errOut, "wikijs> ")
			if !scanner.Scan() {
				return scanner.Err()
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if line == "exit" || line == "quit" {
				return nil
			}
			parts, err := splitCommandLine(line)
			if err != nil {
				fmt.Fprintln(a.errOut, FormatError(err))
				continue
			}
			child := newRootCommand(a)
			child.SetArgs(parts)
			if err := child.ExecuteContext(cmd.Context()); err != nil {
				fmt.Fprintln(a.errOut, FormatError(err))
			}
		}
	}}
}

func (a *app) renderTemplate(name string, values map[string]string) (string, error) {
	content, err := loadTemplate(name)
	if err != nil {
		return "", err
	}
	for key, value := range values {
		content = strings.ReplaceAll(content, "{{"+key+"}}", value)
	}
	return content, nil
}

func (a *app) printSummary(action string, summary operationSummary) error {
	if a.format == "json" {
		return output.JSON(a.out, successResult{Success: true, Action: action, Result: summary})
	}
	_, err := fmt.Fprintf(a.out, "%s complete: %d created, %d updated, %d skipped, %d matched, %d changed\n",
		action, summary.Created, summary.Updated, summary.Skipped, summary.Matched, summary.Changed)
	return err
}

func (a *app) progress(label string, current, total int) {
	if total <= 0 {
		return
	}
	fmt.Fprintf(a.errOut, "\r%s: %d/%d", label, current, total)
}

func (a *app) progressDone() {
	fmt.Fprintln(a.errOut)
}

func markdownFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".md") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func pagePathFromFile(root, file, prefix string) (string, error) {
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return "", err
	}
	rel = strings.TrimSuffix(rel, filepath.Ext(rel))
	rel = filepath.ToSlash(rel)
	return strings.Trim(strings.Trim(prefix, "/")+"/"+rel, "/"), nil
}

func titleFromMarkdown(content, path string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	base := filepath.Base(path)
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	if base == "." || base == "/" || base == "" {
		return "Untitled"
	}
	return strings.Title(base)
}

func templateDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "wikijs", "templates"), nil
	}
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "wikijs", "templates"), nil
}

func templatePath(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, `/\`) {
		return "", errors.New("template name must not contain path separators")
	}
	dir, err := templateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".md"), nil
}

func listTemplates() ([]string, error) {
	dir, err := templateDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), ".md"))
	}
	sort.Strings(names)
	return names, nil
}

func loadTemplate(name string) (string, error) {
	path, err := templatePath(name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	return string(data), err
}

func saveTemplate(name, content string) error {
	path, err := templatePath(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func deleteTemplate(name string) error {
	path, err := templatePath(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func readJSONFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func newReplacer(oldText, newText string, regexMode, caseSensitive bool) (func(string) (string, bool), error) {
	if oldText == "" {
		return nil, errors.New("old text must not be empty")
	}
	if regexMode || !caseSensitive {
		pattern := oldText
		if !regexMode {
			pattern = regexp.QuoteMeta(oldText)
		}
		if !caseSensitive {
			pattern = "(?i)" + pattern
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		return func(content string) (string, bool) {
			if !re.MatchString(content) {
				return content, false
			}
			return re.ReplaceAllString(content, newText), true
		}, nil
	}
	return func(content string) (string, bool) {
		if !strings.Contains(content, oldText) {
			return content, false
		}
		return strings.ReplaceAll(content, oldText, newText), true
	}, nil
}

func splitCommandLine(line string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	escaped := false
	for _, r := range line {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, errors.New("unterminated quote")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, nil
}

func strconvItoa(value int) string {
	return fmt.Sprintf("%d", value)
}
