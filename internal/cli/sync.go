package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dfuentes87/wikijs-cli/internal/api"
	"github.com/dfuentes87/wikijs-cli/internal/config"
	"github.com/dfuentes87/wikijs-cli/internal/output"
)

func (a *app) syncCommand() *cobra.Command {
	var outputPath string
	var fileFormat string
	var pathPrefix string
	var deleteStale bool
	cmd := &cobra.Command{Use: "sync", Short: "Sync pages to a local directory", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if fileFormat != "markdown" && fileFormat != "json" {
			return fmt.Errorf("unsupported file format %q", fileFormat)
		}
		if outputPath == "" {
			cfg, _, err := config.Load(a.configPath)
			if err != nil {
				return err
			}
			outputPath = cfg.AutoSync.Path
		}
		if outputPath == "" {
			return errors.New("sync output path is required; use --output or set autoSync.path in config")
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		pages, err := client.ListPages(cmd.Context(), api.ListOptions{Limit: 0})
		if err != nil {
			return err
		}
		summary := operationSummary{}
		expected := map[string]struct{}{}
		for i, listed := range pages {
			a.progress("Syncing", i+1, len(pages))
			if !hasWikiPathPrefix(listed.Path, pathPrefix) {
				continue
			}
			page, err := client.GetPage(cmd.Context(), strconvItoa(listed.ID), "", false)
			if err != nil {
				return err
			}
			outPath, err := exportFilePath(outputPath, page.Path, fileFormat)
			if err != nil {
				return err
			}
			expected[filepath.Clean(outPath)] = struct{}{}
			changed, existed, err := syncWritePage(outPath, page, fileFormat)
			if err != nil {
				return err
			}
			summary.Pages++
			summary.Files++
			if !changed {
				summary.Skipped++
				continue
			}
			if existed {
				summary.Updated++
			} else {
				summary.Created++
			}
			summary.Paths = append(summary.Paths, outPath)
		}
		if deleteStale {
			deleted, paths, err := deleteStaleSyncFiles(outputPath, fileFormat, pathPrefix, expected)
			if err != nil {
				return err
			}
			summary.Deleted += deleted
			summary.Paths = append(summary.Paths, paths...)
		}
		a.progressDone()
		if a.format == "json" {
			return output.JSON(a.out, successResult{Success: true, Action: "sync", Path: outputPath, Result: summary})
		}
		_, err = fmt.Fprintf(a.out, "Sync complete: %d created, %d updated, %d skipped, %d deleted\n", summary.Created, summary.Updated, summary.Skipped, summary.Deleted)
		return err
	}}
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "sync output directory")
	cmd.Flags().StringVar(&fileFormat, "file-format", "markdown", "sync file format: markdown or json")
	cmd.Flags().StringVar(&pathPrefix, "path", "", "only sync pages under this path")
	cmd.Flags().BoolVar(&deleteStale, "delete", false, "delete stale synced files for the selected file format")
	return cmd
}

func syncWritePage(path string, page api.Page, fileFormat string) (changed bool, existed bool, err error) {
	if _, err := os.Stat(path); err == nil {
		existed = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, false, err
	}
	data, err := pageExportData(page, fileFormat)
	if err != nil {
		return false, false, err
	}
	changed, err = writeFileIfChanged(path, data)
	return changed, existed, err
}

func deleteStaleSyncFiles(root, fileFormat, pathPrefix string, expected map[string]struct{}) (int, []string, error) {
	ext := ".md"
	if fileFormat == "json" {
		ext = ".json"
	}
	var deleted int
	var paths []string
	if _, err := os.Stat(root); errors.Is(err, fs.ErrNotExist) {
		return 0, nil, nil
	} else if err != nil {
		return 0, nil, err
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 || filepath.Ext(path) != ext {
			return nil
		}
		if pathPrefix != "" {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			wikiPath := normalizeWikiPath(filepath.ToSlash(rel[:len(rel)-len(ext)]))
			if !hasWikiPathPrefix(wikiPath, pathPrefix) {
				return nil
			}
		}
		clean := filepath.Clean(path)
		if _, ok := expected[clean]; ok {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		deleted++
		paths = append(paths, path)
		return nil
	})
	return deleted, paths, err
}
