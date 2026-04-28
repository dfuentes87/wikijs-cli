package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dfuentes87/wikijs-cli/internal/api"
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
