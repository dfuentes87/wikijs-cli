package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dfuentes87/wikijs-cli/internal/api"
	"github.com/dfuentes87/wikijs-cli/internal/output"
)

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

type bulkMoveItem struct {
	ID     int    `json:"id"`
	Locale string `json:"locale,omitempty"`
	From   string `json:"from"`
	To     string `json:"to"`
}

type bulkMoveResult struct {
	Matched int            `json:"matched"`
	Moved   int            `json:"moved"`
	Moves   []bulkMoveItem `json:"moves"`
}

func (a *app) bulkMoveCommand() *cobra.Command {
	var locale string
	var dryRun, force bool
	cmd := &cobra.Command{Use: "bulk-move <from-prefix> <to-prefix>", Short: "Move pages under a path prefix", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		fromPrefix := normalizeWikiPath(args[0])
		toPrefix := normalizeWikiPath(args[1])
		if fromPrefix == "" {
			return errors.New("from-prefix must not be empty")
		}
		if fromPrefix == toPrefix {
			return errors.New("from-prefix and to-prefix must be different")
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		pages, err := client.ListPages(cmd.Context(), api.ListOptions{Limit: 0})
		if err != nil {
			return err
		}
		result, err := planBulkMove(pages, fromPrefix, toPrefix, locale)
		if err != nil {
			return err
		}
		if a.format == "json" {
			if !dryRun && len(result.Moves) > 0 {
				if !force && !a.confirm(fmt.Sprintf("Move %d pages from %s to %s? This changes live content.", len(result.Moves), fromPrefix, toPrefix)) {
					return errors.New("bulk move cancelled")
				}
				if err := applyBulkMove(cmd.Context(), client, result.Moves, a); err != nil {
					return err
				}
				result.Moved = len(result.Moves)
			}
			return output.JSON(a.out, successResult{Success: true, Action: "bulk-move", Result: result})
		}
		if err := printBulkMovePlan(a.out, result.Moves); err != nil {
			return err
		}
		if dryRun || len(result.Moves) == 0 {
			return printBulkMoveSummary(a.out, a, "bulk-move", result)
		}
		if !force && !a.confirm(fmt.Sprintf("Move %d pages from %s to %s? This changes live content.", len(result.Moves), fromPrefix, toPrefix)) {
			return errors.New("bulk move cancelled")
		}
		if err := applyBulkMove(cmd.Context(), client, result.Moves, a); err != nil {
			return err
		}
		result.Moved = len(result.Moves)
		return printBulkMoveSummary(a.out, a, "bulk-move", result)
	}}
	cmd.Flags().StringVar(&locale, "locale", "", "destination locale")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show pages without moving them")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")
	return cmd
}

type bulkTagItem struct {
	ID          int      `json:"id"`
	Path        string   `json:"path"`
	Locale      string   `json:"locale,omitempty"`
	CurrentTags []string `json:"currentTags"`
	NewTags     []string `json:"newTags"`
	Changed     bool     `json:"changed"`
}

type bulkTagResult struct {
	Matched int           `json:"matched"`
	Changed int           `json:"changed"`
	Pages   []bulkTagItem `json:"pages"`
}

func (a *app) bulkTagCommand() *cobra.Command {
	var locale string
	var dryRun bool
	cmd := &cobra.Command{Use: "bulk-tag <path-prefix> <add|remove|set> <tags>", Short: "Manage tags for pages under a path prefix", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		pathPrefix := normalizeWikiPath(args[0])
		if pathPrefix == "" {
			return errors.New("path-prefix must not be empty")
		}
		tags := parseTags(args[2])
		if len(tags) == 0 {
			return errors.New("at least one tag is required")
		}
		if _, err := tagsAfterOperation(nil, args[1], tags); err != nil {
			return err
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		pages, err := client.ListPages(cmd.Context(), api.ListOptions{Limit: 0})
		if err != nil {
			return err
		}
		result, err := planBulkTag(pages, pathPrefix, locale, args[1], tags)
		if err != nil {
			return err
		}
		if !dryRun {
			for i, item := range result.Pages {
				a.progress("Tagging", i+1, len(result.Pages))
				if !item.Changed {
					continue
				}
				if _, err := client.UpdatePage(cmd.Context(), api.UpdatePageInput{ID: item.ID, Tags: item.NewTags, SetTags: true}); err != nil {
					return err
				}
			}
			a.progressDone()
		}
		if a.format == "json" {
			return output.JSON(a.out, successResult{Success: true, Action: "bulk-tag", Result: result})
		}
		if err := printBulkTagPlan(a.out, result.Pages); err != nil {
			return err
		}
		_, err = fmt.Fprintf(a.out, "%s\n", a.success(fmt.Sprintf("bulk-tag complete: %d matched, %d changed", result.Matched, result.Changed)))
		return err
	}}
	cmd.Flags().StringVar(&locale, "locale", "", "only tag pages in this locale")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show pages without updating tags")
	return cmd
}

type bulkDeleteItem struct {
	ID     int    `json:"id"`
	Path   string `json:"path"`
	Title  string `json:"title,omitempty"`
	Locale string `json:"locale,omitempty"`
}

type bulkDeleteResult struct {
	Matched int              `json:"matched"`
	Deleted int              `json:"deleted"`
	Pages   []bulkDeleteItem `json:"pages"`
}

func (a *app) bulkDeleteCommand() *cobra.Command {
	var locale string
	var dryRun, force bool
	cmd := &cobra.Command{Use: "bulk-delete <path-prefix>", Short: "Delete pages under a path prefix", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		pathPrefix := normalizeWikiPath(args[0])
		if pathPrefix == "" {
			return errors.New("path-prefix must not be empty")
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		pages, err := client.ListPages(cmd.Context(), api.ListOptions{Limit: 0})
		if err != nil {
			return err
		}
		result := planBulkDelete(pages, pathPrefix, locale)
		if a.format == "json" {
			if !dryRun && len(result.Pages) > 0 {
				if !force && !a.confirm(fmt.Sprintf("Delete %d pages under %s? This cannot be undone.", len(result.Pages), pathPrefix)) {
					return errors.New("bulk delete cancelled")
				}
				if err := applyBulkDelete(cmd.Context(), client, result.Pages, a); err != nil {
					return err
				}
				result.Deleted = len(result.Pages)
			}
			return output.JSON(a.out, successResult{Success: true, Action: "bulk-delete", Result: result})
		}
		if err := printBulkDeletePlan(a.out, result.Pages); err != nil {
			return err
		}
		if dryRun || len(result.Pages) == 0 {
			return printBulkDeleteSummary(a.out, a, result)
		}
		if !force && !a.confirm(fmt.Sprintf("Delete %d pages under %s? This cannot be undone.", len(result.Pages), pathPrefix)) {
			return errors.New("bulk delete cancelled")
		}
		if err := applyBulkDelete(cmd.Context(), client, result.Pages, a); err != nil {
			return err
		}
		result.Deleted = len(result.Pages)
		return printBulkDeleteSummary(a.out, a, result)
	}}
	cmd.Flags().StringVar(&locale, "locale", "", "only delete pages in this locale")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show pages without deleting them")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")
	return cmd
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

func planBulkMove(pages []api.Page, fromPrefix, toPrefix, locale string) (bulkMoveResult, error) {
	fromPrefix = normalizeWikiPath(fromPrefix)
	toPrefix = normalizeWikiPath(toPrefix)
	moves := make([]bulkMoveItem, 0)
	for _, page := range pages {
		if !hasWikiPathPrefix(page.Path, fromPrefix) {
			continue
		}
		itemLocale := page.Locale
		if locale != "" {
			itemLocale = locale
		}
		toPath := bulkMoveDestination(page.Path, fromPrefix, toPrefix)
		if toPath == "" {
			return bulkMoveResult{}, fmt.Errorf("destination path for %s is empty", page.Path)
		}
		moves = append(moves, bulkMoveItem{ID: page.ID, Locale: itemLocale, From: normalizeWikiPath(page.Path), To: toPath})
	}
	sort.Slice(moves, func(i, j int) bool {
		if moves[i].From == moves[j].From {
			return moves[i].Locale < moves[j].Locale
		}
		return moves[i].From < moves[j].From
	})
	if err := validateBulkMoveDestinations(pages, moves); err != nil {
		return bulkMoveResult{}, err
	}
	return bulkMoveResult{Matched: len(moves), Moves: moves}, nil
}

func bulkMoveDestination(pagePath, fromPrefix, toPrefix string) string {
	pagePath = normalizeWikiPath(pagePath)
	fromPrefix = normalizeWikiPath(fromPrefix)
	toPrefix = normalizeWikiPath(toPrefix)
	if pagePath == fromPrefix {
		return toPrefix
	}
	suffix := strings.TrimPrefix(pagePath, fromPrefix+"/")
	if toPrefix == "" {
		return suffix
	}
	return strings.Trim(toPrefix+"/"+suffix, "/")
}

func validateBulkMoveDestinations(pages []api.Page, moves []bulkMoveItem) error {
	current := map[string]int{}
	for _, page := range pages {
		current[bulkMoveKey(page.Locale, page.Path)] = page.ID
	}
	seen := map[string]bulkMoveItem{}
	for _, move := range moves {
		key := bulkMoveKey(move.Locale, move.To)
		if previous, ok := seen[key]; ok {
			return fmt.Errorf("destination collision: pages %d and %d both move to %s", previous.ID, move.ID, move.To)
		}
		seen[key] = move
		if existingID, ok := current[key]; ok && existingID != move.ID {
			return fmt.Errorf("destination collision: %s already exists", move.To)
		}
	}
	return nil
}

func bulkMoveKey(locale, path string) string {
	return locale + "\x00" + normalizeWikiPath(path)
}

func applyBulkMove(ctx context.Context, client WikiClient, moves []bulkMoveItem, a *app) error {
	for i, move := range moves {
		a.progress("Moving", i+1, len(moves))
		if err := client.MovePage(ctx, move.ID, move.To, move.Locale); err != nil {
			return err
		}
	}
	a.progressDone()
	return nil
}

func printBulkMovePlan(w io.Writer, moves []bulkMoveItem) error {
	rows := make([][]string, 0, len(moves))
	for _, move := range moves {
		rows = append(rows, []string{strconvItoa(move.ID), move.Locale, move.From, move.To})
	}
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "No pages matched")
		return err
	}
	return output.Table(w, []string{"ID", "Locale", "From", "To"}, rows)
}

func printBulkMoveSummary(w io.Writer, a *app, action string, result bulkMoveResult) error {
	_, err := fmt.Fprintf(w, "%s\n", a.success(fmt.Sprintf("%s complete: %d matched, %d moved", action, result.Matched, result.Moved)))
	return err
}

func planBulkTag(pages []api.Page, pathPrefix, locale, operation string, tags []string) (bulkTagResult, error) {
	items := make([]bulkTagItem, 0)
	for _, page := range pages {
		if !hasWikiPathPrefix(page.Path, pathPrefix) || (locale != "" && page.Locale != locale) {
			continue
		}
		current := append([]string(nil), []string(page.Tags)...)
		sort.Strings(current)
		next, err := tagsAfterOperation(current, operation, tags)
		if err != nil {
			return bulkTagResult{}, err
		}
		items = append(items, bulkTagItem{
			ID:          page.ID,
			Path:        normalizeWikiPath(page.Path),
			Locale:      page.Locale,
			CurrentTags: current,
			NewTags:     next,
			Changed:     !sameStrings(current, next),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Locale < items[j].Locale
		}
		return items[i].Path < items[j].Path
	})
	result := bulkTagResult{Matched: len(items), Pages: items}
	for _, item := range items {
		if item.Changed {
			result.Changed++
		}
	}
	return result, nil
}

func printBulkTagPlan(w io.Writer, items []bulkTagItem) error {
	if len(items) == 0 {
		_, err := fmt.Fprintln(w, "No pages matched")
		return err
	}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			strconvItoa(item.ID),
			item.Path,
			item.Locale,
			strings.Join(item.CurrentTags, ", "),
			strings.Join(item.NewTags, ", "),
			output.Bool(item.Changed),
		})
	}
	return output.Table(w, []string{"ID", "Page", "Locale", "Current Tags", "New Tags", "Changed"}, rows)
}

func planBulkDelete(pages []api.Page, pathPrefix, locale string) bulkDeleteResult {
	items := make([]bulkDeleteItem, 0)
	for _, page := range pages {
		if !hasWikiPathPrefix(page.Path, pathPrefix) || (locale != "" && page.Locale != locale) {
			continue
		}
		items = append(items, bulkDeleteItem{ID: page.ID, Path: normalizeWikiPath(page.Path), Title: page.Title, Locale: page.Locale})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Locale < items[j].Locale
		}
		return items[i].Path < items[j].Path
	})
	return bulkDeleteResult{Matched: len(items), Pages: items}
}

func applyBulkDelete(ctx context.Context, client WikiClient, items []bulkDeleteItem, a *app) error {
	for i, item := range items {
		a.progress("Deleting", i+1, len(items))
		if err := client.DeletePage(ctx, item.ID); err != nil {
			return err
		}
	}
	a.progressDone()
	return nil
}

func printBulkDeletePlan(w io.Writer, items []bulkDeleteItem) error {
	if len(items) == 0 {
		_, err := fmt.Fprintln(w, "No pages matched")
		return err
	}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{strconvItoa(item.ID), item.Path, item.Locale, item.Title})
	}
	return output.Table(w, []string{"ID", "Page", "Locale", "Title"}, rows)
}

func printBulkDeleteSummary(w io.Writer, a *app, result bulkDeleteResult) error {
	_, err := fmt.Fprintf(w, "%s\n", a.success(fmt.Sprintf("bulk-delete complete: %d matched, %d deleted", result.Matched, result.Deleted)))
	return err
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
