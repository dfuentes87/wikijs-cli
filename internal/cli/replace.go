package cli

import (
	"errors"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dfuentes87/wikijs-cli/internal/api"
)

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
			a.progress("Scanning", i+1, len(pages))
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
