package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	pathpkg "path"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dfuentes87/wikijs-cli/internal/api"
	"github.com/dfuentes87/wikijs-cli/internal/markdown"
	"github.com/dfuentes87/wikijs-cli/internal/output"
)

type brokenLink struct {
	PageID   int    `json:"pageId"`
	PagePath string `json:"pagePath"`
	Line     int    `json:"line"`
	Target   string `json:"target"`
	Resolved string `json:"resolved"`
}

type linkCheckResult struct {
	Valid   bool         `json:"valid"`
	Checked int          `json:"checked"`
	Broken  []brokenLink `json:"broken"`
}

type brokenImage struct {
	PageID   int    `json:"pageId"`
	PagePath string `json:"pagePath"`
	Line     int    `json:"line"`
	Target   string `json:"target"`
	Resolved string `json:"resolved"`
}

type validationIssue struct {
	PageID   int    `json:"pageId"`
	PagePath string `json:"pagePath"`
	Line     int    `json:"line"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

type validationResult struct {
	Valid        bool              `json:"valid"`
	Pages        int               `json:"pages"`
	Errors       []validationIssue `json:"errors,omitempty"`
	Warnings     []validationIssue `json:"warnings,omitempty"`
	BrokenLinks  []brokenLink      `json:"brokenLinks,omitempty"`
	BrokenImages []brokenImage     `json:"brokenImages,omitempty"`
}

func (a *app) checkLinksCommand() *cobra.Command {
	var pathPrefix string
	cmd := &cobra.Command{Use: "check-links", Short: "Find broken internal links", RunE: func(cmd *cobra.Command, args []string) error {
		client, err := a.getClient()
		if err != nil {
			return err
		}
		result, err := a.checkLinks(cmd.Context(), client, pathPrefix)
		if err != nil {
			return err
		}
		if a.format == "json" {
			if err := output.JSON(a.out, result); err != nil {
				return err
			}
		} else {
			rows := make([][]string, 0, len(result.Broken))
			for _, item := range result.Broken {
				rows = append(rows, []string{item.PagePath, strconv.Itoa(item.Line), item.Target, item.Resolved})
			}
			if len(rows) == 0 {
				fmt.Fprintf(a.out, "Checked %d pages; no broken internal links found\n", result.Checked)
			} else if err := output.Table(a.out, []string{"Page", "Line", "Target", "Resolved"}, rows); err != nil {
				return err
			}
		}
		if !result.Valid {
			return errors.New("broken internal links found")
		}
		return nil
	}}
	cmd.Flags().StringVar(&pathPrefix, "path", "", "only check pages under this path")
	return cmd
}

func (a *app) validateCommand() *cobra.Command {
	var all bool
	var pathPrefix string
	cmd := &cobra.Command{Use: "validate [id-or-path]", Short: "Validate page content", Args: cobra.ArbitraryArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if all && len(args) != 0 {
			return errors.New("use either --all or an id/path argument")
		}
		if !all && len(args) != 1 {
			return errors.New("validate requires an id/path argument or --all")
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		pages, err := pagesForValidation(cmd.Context(), client, all, args, pathPrefix)
		if err != nil {
			return err
		}
		assets, err := client.ListAssets(cmd.Context(), "", 0)
		if err != nil {
			return err
		}
		allPages, err := client.ListPages(cmd.Context(), api.ListOptions{Limit: 0})
		if err != nil {
			return err
		}
		result := validatePages(pages, pagePathSet(allPages), assetPathSet(assets))
		if a.format == "json" {
			if err := output.JSON(a.out, result); err != nil {
				return err
			}
		} else if err := printValidationResult(a.out, result); err != nil {
			return err
		}
		if !result.Valid {
			return errors.New("validation failed")
		}
		return nil
	}}
	cmd.Flags().BoolVar(&all, "all", false, "validate all pages")
	cmd.Flags().StringVar(&pathPrefix, "path", "", "only validate pages under this path with --all")
	return cmd
}

func (a *app) checkLinks(ctx context.Context, client WikiClient, pathPrefix string) (linkCheckResult, error) {
	listed, err := client.ListPages(ctx, api.ListOptions{Limit: 0})
	if err != nil {
		return linkCheckResult{}, err
	}
	existing := pagePathSet(listed)
	result := linkCheckResult{Valid: true}
	for i, page := range listed {
		a.progress("Checking links", i+1, len(listed))
		if !hasWikiPathPrefix(page.Path, pathPrefix) {
			continue
		}
		fullPage, err := client.GetPage(ctx, strconvItoa(page.ID), "", false)
		if err != nil {
			return linkCheckResult{}, err
		}
		result.Checked++
		result.Broken = append(result.Broken, brokenLinksForPage(fullPage, existing)...)
	}
	a.progressDone()
	result.Valid = len(result.Broken) == 0
	return result, nil
}

func pagesForValidation(ctx context.Context, client WikiClient, all bool, args []string, pathPrefix string) ([]api.Page, error) {
	if !all {
		page, err := client.GetPage(ctx, args[0], "", false)
		if err != nil {
			return nil, err
		}
		return []api.Page{page}, nil
	}
	listed, err := client.ListPages(ctx, api.ListOptions{Limit: 0})
	if err != nil {
		return nil, err
	}
	pages := make([]api.Page, 0, len(listed))
	for _, page := range listed {
		if !hasWikiPathPrefix(page.Path, pathPrefix) {
			continue
		}
		fullPage, err := client.GetPage(ctx, strconvItoa(page.ID), "", false)
		if err != nil {
			return nil, err
		}
		pages = append(pages, fullPage)
	}
	return pages, nil
}

func validatePages(pages []api.Page, existingPages map[string]struct{}, assets map[string]struct{}) validationResult {
	result := validationResult{Valid: true, Pages: len(pages)}
	for _, page := range pages {
		lint := markdown.Lint(page.Content)
		for _, issue := range lint.Errors {
			result.Errors = append(result.Errors, validationIssue{PageID: page.ID, PagePath: page.Path, Line: issue.Line, Rule: issue.Rule, Message: issue.Message})
		}
		for _, issue := range lint.Warnings {
			result.Warnings = append(result.Warnings, validationIssue{PageID: page.ID, PagePath: page.Path, Line: issue.Line, Rule: issue.Rule, Message: issue.Message})
		}
		result.BrokenLinks = append(result.BrokenLinks, brokenLinksForPage(page, existingPages)...)
		result.BrokenImages = append(result.BrokenImages, brokenImagesForPage(page, assets)...)
	}
	result.Valid = len(result.Errors) == 0 && len(result.BrokenLinks) == 0 && len(result.BrokenImages) == 0
	return result
}

func brokenLinksForPage(page api.Page, existing map[string]struct{}) []brokenLink {
	var broken []brokenLink
	for _, link := range markdown.Links(page.Content) {
		if link.Image {
			continue
		}
		resolved, ok := internalPageTarget(page.Path, link.Target)
		if !ok {
			continue
		}
		if _, exists := existing[resolved]; !exists {
			broken = append(broken, brokenLink{PageID: page.ID, PagePath: page.Path, Line: link.Line, Target: link.Target, Resolved: resolved})
		}
	}
	return broken
}

func brokenImagesForPage(page api.Page, assets map[string]struct{}) []brokenImage {
	var broken []brokenImage
	for _, link := range markdown.Links(page.Content) {
		if !link.Image {
			continue
		}
		resolved, ok := internalAssetTarget(link.Target)
		if !ok {
			continue
		}
		if _, exists := assets[resolved]; exists {
			continue
		}
		if _, exists := assets[pathpkg.Base(resolved)]; exists {
			continue
		}
		broken = append(broken, brokenImage{PageID: page.ID, PagePath: page.Path, Line: link.Line, Target: link.Target, Resolved: resolved})
	}
	return broken
}

func pagePathSet(pages []api.Page) map[string]struct{} {
	paths := make(map[string]struct{}, len(pages))
	for _, page := range pages {
		paths[normalizeWikiPath(page.Path)] = struct{}{}
	}
	return paths
}

func assetPathSet(assets []api.Asset) map[string]struct{} {
	paths := make(map[string]struct{}, len(assets)*2)
	for _, asset := range assets {
		clean := normalizeWikiPath(asset.Filename)
		paths[clean] = struct{}{}
		if base := pathpkg.Base(clean); base != "." && base != "/" {
			paths[base] = struct{}{}
		}
	}
	return paths
}

func internalPageTarget(sourcePath, target string) (string, bool) {
	target = stripTargetFragment(target)
	if target == "" || strings.HasPrefix(target, "#") || isExternalTarget(target) {
		return "", false
	}
	if strings.HasPrefix(target, "/") {
		return normalizeWikiPath(target), true
	}
	base := pathpkg.Dir("/" + normalizeWikiPath(sourcePath))
	return normalizeWikiPath(pathpkg.Join(base, target)), true
}

func internalAssetTarget(target string) (string, bool) {
	target = stripTargetFragment(target)
	if target == "" || strings.HasPrefix(target, "#") || isExternalTarget(target) {
		return "", false
	}
	return normalizeWikiPath(target), true
}

func stripTargetFragment(target string) string {
	target = strings.TrimSpace(target)
	if idx := strings.IndexAny(target, "?#"); idx >= 0 {
		target = target[:idx]
	}
	return target
}

func isExternalTarget(target string) bool {
	if strings.HasPrefix(target, "//") {
		return true
	}
	parsed, err := url.Parse(target)
	return err == nil && parsed.Scheme != ""
}

func normalizeWikiPath(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "/")
	value = pathpkg.Clean("/" + value)
	value = strings.TrimPrefix(value, "/")
	value = strings.TrimSuffix(value, "/")
	value = strings.TrimSuffix(value, ".md")
	if strings.HasSuffix(value, "/index") {
		value = strings.TrimSuffix(value, "/index")
	}
	if value == "." {
		return ""
	}
	return value
}

func hasWikiPathPrefix(pagePath, prefix string) bool {
	prefix = normalizeWikiPath(prefix)
	if prefix == "" {
		return true
	}
	pagePath = normalizeWikiPath(pagePath)
	return pagePath == prefix || strings.HasPrefix(pagePath, prefix+"/")
}

func printValidationResult(w io.Writer, result validationResult) error {
	if result.Valid {
		_, err := fmt.Fprintf(w, "Validated %d pages; no errors found\n", result.Pages)
		return err
	}
	var lines []string
	for _, issue := range result.Errors {
		lines = append(lines, fmt.Sprintf("%s:%d %s: %s", issue.PagePath, issue.Line, issue.Rule, issue.Message))
	}
	for _, link := range result.BrokenLinks {
		lines = append(lines, fmt.Sprintf("%s:%d broken link %s -> %s", link.PagePath, link.Line, link.Target, link.Resolved))
	}
	for _, image := range result.BrokenImages {
		lines = append(lines, fmt.Sprintf("%s:%d broken image %s -> %s", image.PagePath, image.Line, image.Target, image.Resolved))
	}
	sort.Strings(lines)
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	if len(result.Warnings) > 0 {
		fmt.Fprintf(w, "%d warnings\n", len(result.Warnings))
	}
	return nil
}
