package cli

import (
	"encoding/json"
	"errors"
	"fmt"
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
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return err
			}
			if fileFormat == "json" {
				data, err := json.MarshalIndent(page, "", "  ")
				if err != nil {
					return err
				}
				if err := os.WriteFile(outPath, append(data, '\n'), 0o600); err != nil {
					return err
				}
			} else if err := os.WriteFile(outPath, []byte(page.Content), 0o600); err != nil {
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
		_, err = fmt.Fprintf(a.out, "Exported %d pages to %s\n", summary.Pages, args[0])
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
