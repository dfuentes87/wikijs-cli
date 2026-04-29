package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		_, err = fmt.Fprintln(a.out, a.success(fmt.Sprintf("Backed up %d pages to %s", len(fullPages), outputPath)))
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
		_, err = fmt.Fprintln(a.out, a.success(fmt.Sprintf("Restore complete: %d created, %d updated, %d skipped", summary.Created, summary.Updated, summary.Skipped)))
		return err
	}}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be restored without changing Wiki.js")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "skip pages that already exist")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing pages")
	return cmd
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
