package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dfuentes87/wikijs-cli/internal/api"
	"github.com/dfuentes87/wikijs-cli/internal/output"
)

func (a *app) exportCommand() *cobra.Command {
	var fileFormat string
	var pathPrefix string
	cmd := &cobra.Command{Use: "export <directory>", Short: "Export pages to files", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if fileFormat != "markdown" && fileFormat != "json" {
			return fmt.Errorf("unsupported file format %q", fileFormat)
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
		for i, listed := range pages {
			a.progress("Exporting", i+1, len(pages))
			if !hasWikiPathPrefix(listed.Path, pathPrefix) {
				continue
			}
			page, err := client.GetPage(cmd.Context(), strconvItoa(listed.ID), "", false)
			if err != nil {
				return err
			}
			outPath, err := exportFilePath(args[0], page.Path, fileFormat)
			if err != nil {
				return err
			}
			data, err := pageExportData(page, fileFormat)
			if err != nil {
				return err
			}
			if _, err := writeFileIfChanged(outPath, data); err != nil {
				return err
			}
			summary.Pages++
			summary.Files++
			summary.Paths = append(summary.Paths, outPath)
		}
		a.progressDone()
		if a.format == "json" {
			return output.JSON(a.out, successResult{Success: true, Action: "export", Path: args[0], Result: summary})
		}
		_, err = fmt.Fprintln(a.out, a.success(fmt.Sprintf("Exported %d pages to %s", summary.Pages, args[0])))
		return err
	}}
	cmd.Flags().StringVar(&fileFormat, "file-format", "markdown", "export file format: markdown or json")
	cmd.Flags().StringVar(&pathPrefix, "path", "", "only export pages under this path")
	return cmd
}

func exportFilePath(root, pagePath, fileFormat string) (string, error) {
	clean := normalizeWikiPath(pagePath)
	if clean == "" || clean == "." {
		clean = "index"
	}
	rel := filepath.Clean(filepath.FromSlash(clean))
	if rel == "." || rel == string(filepath.Separator) || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", errors.New("invalid page path for export")
	}
	ext := ".md"
	if fileFormat == "json" {
		ext = ".json"
	}
	if filepath.Ext(rel) != ext {
		rel += ext
	}
	return filepath.Join(root, rel), nil
}

func pageExportData(page api.Page, fileFormat string) ([]byte, error) {
	if fileFormat == "json" {
		data, err := json.MarshalIndent(page, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(data, '\n'), nil
	}
	return []byte(page.Content), nil
}

func writeFileIfChanged(path string, data []byte) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, data) {
		return false, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, data, 0o600)
}
