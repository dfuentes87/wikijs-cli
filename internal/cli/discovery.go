package cli

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dfuentes87/wikijs-cli/internal/api"
	"github.com/dfuentes87/wikijs-cli/internal/output"
)

type grepMatch struct {
	PageID   int    `json:"pageId"`
	PagePath string `json:"pagePath"`
	Title    string `json:"title"`
	Line     int    `json:"line"`
	Text     string `json:"text"`
}

type grepResult struct {
	Pattern string      `json:"pattern"`
	Matched int         `json:"matched"`
	Matches []grepMatch `json:"matches"`
}

type detailedStats struct {
	api.Stats
	Words              int            `json:"words"`
	EstimatedReadMins  int            `json:"estimatedReadMins"`
	AverageWordsByPage int            `json:"averageWordsByPage"`
	TopPaths           map[string]int `json:"topPaths"`
}

func (a *app) tagCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "tag <id> <add|remove|set> <tags>", Short: "Manage page tags", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}
		tags := parseTags(args[2])
		if len(tags) == 0 {
			return errors.New("at least one tag is required")
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		page, err := applyTagOperation(cmd.Context(), client, id, args[1], tags)
		if err != nil {
			return err
		}
		if a.format == "json" {
			return output.JSON(a.out, successResult{Success: true, Action: "tag_" + args[1], ID: id, Result: page})
		}
		_, err = fmt.Fprintln(a.out, a.success(fmt.Sprintf("Updated tags for page %d: %s", id, strings.Join([]string(page.Tags), ", "))))
		return err
	}}
	return cmd
}

func (a *app) infoCommand() *cobra.Command {
	var locale string
	cmd := &cobra.Command{Use: "info <id-or-path>", Short: "Show page metadata", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client, err := a.getClient()
		if err != nil {
			return err
		}
		page, err := client.GetPage(cmd.Context(), args[0], locale, true)
		if err != nil {
			return err
		}
		rows := [][]string{
			{"ID", strconv.Itoa(page.ID)},
			{"Path", page.Path},
			{"Title", page.Title},
			{"Description", page.Description},
			{"Locale", page.Locale},
			{"Author", page.AuthorName},
			{"Published", output.Bool(page.IsPublished)},
			{"Private", output.Bool(page.IsPrivate)},
			{"Created", output.Date(page.CreatedAt)},
			{"Updated", output.Date(page.UpdatedAt)},
			{"Tags", strings.Join([]string(page.Tags), ", ")},
			{"Children", strconv.Itoa(len(page.Children))},
		}
		return a.print(page, []string{"Field", "Value"}, rows)
	}}
	cmd.Flags().StringVar(&locale, "locale", "", "page locale for path lookups")
	return cmd
}

func (a *app) grepCommand() *cobra.Command {
	var pathPrefix string
	var caseSensitive bool
	var regexMode bool
	var limit int
	cmd := &cobra.Command{Use: "grep <pattern>", Short: "Search within page content", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		matcher, err := newTextMatcher(args[0], regexMode, caseSensitive)
		if err != nil {
			return err
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		result, err := grepPages(cmd.Context(), a, client, args[0], pathPrefix, limit, matcher)
		if err != nil {
			return err
		}
		if a.format == "json" {
			return output.JSON(a.out, result)
		}
		rows := make([][]string, 0, len(result.Matches))
		for _, match := range result.Matches {
			rows = append(rows, []string{match.PagePath, strconv.Itoa(match.Line), match.Text})
		}
		if len(rows) == 0 {
			_, err = fmt.Fprintln(a.out, "No matches found")
			return err
		}
		return output.Table(a.out, []string{"Page", "Line", "Text"}, rows)
	}}
	cmd.Flags().StringVar(&pathPrefix, "path", "", "only search pages under this path")
	cmd.Flags().BoolVar(&caseSensitive, "case-sensitive", false, "use case-sensitive matching")
	cmd.Flags().BoolVar(&regexMode, "regex", false, "treat pattern as a regular expression")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum matches")
	return cmd
}

func (a *app) statsCommand() *cobra.Command {
	var detailed bool
	cmd := &cobra.Command{Use: "stats", Short: "Show page statistics", RunE: func(cmd *cobra.Command, args []string) error {
		client, err := a.getClient()
		if err != nil {
			return err
		}
		stats, err := client.Stats(cmd.Context())
		if err != nil {
			return err
		}
		if !detailed {
			return a.print(stats, []string{"Metric", "Value"}, [][]string{{"Total pages", strconv.Itoa(stats.TotalPages)}, {"Published", strconv.Itoa(stats.PublishedPages)}, {"Drafts", strconv.Itoa(stats.DraftPages)}})
		}
		detail, err := buildDetailedStats(cmd.Context(), a, client, stats)
		if err != nil {
			return err
		}
		rows := [][]string{
			{"Total pages", strconv.Itoa(detail.TotalPages)},
			{"Published", strconv.Itoa(detail.PublishedPages)},
			{"Drafts", strconv.Itoa(detail.DraftPages)},
			{"Words", strconv.Itoa(detail.Words)},
			{"Estimated read minutes", strconv.Itoa(detail.EstimatedReadMins)},
			{"Average words per page", strconv.Itoa(detail.AverageWordsByPage)},
		}
		return a.print(detail, []string{"Metric", "Value"}, rows)
	}}
	cmd.Flags().BoolVar(&detailed, "detailed", false, "include content-derived metrics")
	return cmd
}

func applyTagOperation(ctx context.Context, client WikiClient, id int, operation string, tags []string) (api.Page, error) {
	current, err := client.GetPage(ctx, strconv.Itoa(id), "", false)
	if err != nil {
		return api.Page{}, err
	}
	next := []string(current.Tags)
	switch operation {
	case "add":
		for _, tag := range tags {
			if !containsTag(next, tag) {
				next = append(next, tag)
			}
		}
	case "remove":
		remove := map[string]struct{}{}
		for _, tag := range tags {
			remove[tag] = struct{}{}
		}
		kept := next[:0]
		for _, tag := range next {
			if _, ok := remove[tag]; !ok {
				kept = append(kept, tag)
			}
		}
		next = kept
	case "set":
		next = tags
	default:
		return api.Page{}, fmt.Errorf("unsupported tag operation %q", operation)
	}
	sort.Strings(next)
	return client.UpdatePage(ctx, api.UpdatePageInput{ID: id, Tags: next, SetTags: true})
}

func containsTag(tags []string, value string) bool {
	for _, tag := range tags {
		if tag == value {
			return true
		}
	}
	return false
}

func newTextMatcher(pattern string, regexMode, caseSensitive bool) (func(string) bool, error) {
	if pattern == "" {
		return nil, errors.New("pattern must not be empty")
	}
	if regexMode || !caseSensitive {
		expr := pattern
		if !regexMode {
			expr = regexp.QuoteMeta(pattern)
		}
		if !caseSensitive {
			expr = "(?i)" + expr
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			return nil, err
		}
		return re.MatchString, nil
	}
	return func(line string) bool {
		return strings.Contains(line, pattern)
	}, nil
}

func grepPages(ctx context.Context, a *app, client WikiClient, pattern, pathPrefix string, limit int, matcher func(string) bool) (grepResult, error) {
	pages, err := client.ListPages(ctx, api.ListOptions{Limit: 0})
	if err != nil {
		return grepResult{}, err
	}
	result := grepResult{Pattern: pattern}
	for i, listed := range pages {
		a.progress("Searching", i+1, len(pages))
		if !hasWikiPathPrefix(listed.Path, pathPrefix) {
			continue
		}
		page, err := client.GetPage(ctx, strconvItoa(listed.ID), "", false)
		if err != nil {
			return grepResult{}, err
		}
		for idx, line := range strings.Split(page.Content, "\n") {
			if !matcher(line) {
				continue
			}
			result.Matches = append(result.Matches, grepMatch{PageID: page.ID, PagePath: page.Path, Title: page.Title, Line: idx + 1, Text: strings.TrimSpace(line)})
			result.Matched++
			if limit > 0 && len(result.Matches) >= limit {
				a.progressDone()
				return result, nil
			}
		}
	}
	a.progressDone()
	return result, nil
}

func buildDetailedStats(ctx context.Context, a *app, client WikiClient, stats api.Stats) (detailedStats, error) {
	pages, err := client.ListPages(ctx, api.ListOptions{Limit: 0})
	if err != nil {
		return detailedStats{}, err
	}
	detail := detailedStats{Stats: stats, TopPaths: map[string]int{}}
	for i, listed := range pages {
		a.progress("Calculating stats", i+1, len(pages))
		page, err := client.GetPage(ctx, strconvItoa(listed.ID), "", false)
		if err != nil {
			return detailedStats{}, err
		}
		words := len(strings.Fields(page.Content))
		detail.Words += words
		detail.TopPaths[page.Path] = words
	}
	a.progressDone()
	if detail.TotalPages > 0 {
		detail.AverageWordsByPage = detail.Words / detail.TotalPages
	}
	detail.EstimatedReadMins = (detail.Words + 199) / 200
	return detail, nil
}
