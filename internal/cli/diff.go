package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dfuentes87/wikijs-cli/internal/output"
)

type diffResult struct {
	PageID     int    `json:"pageId"`
	From       string `json:"from"`
	To         string `json:"to"`
	Changed    bool   `json:"changed"`
	Diff       string `json:"diff"`
	FromTitle  string `json:"fromTitle,omitempty"`
	ToTitle    string `json:"toTitle,omitempty"`
	FromAuthor string `json:"fromAuthor,omitempty"`
	ToAuthor   string `json:"toAuthor,omitempty"`
}

func (a *app) diffCommand() *cobra.Command {
	return &cobra.Command{Use: "diff <page-id> [from-version] [to-version]", Short: "Compare page versions", Args: cobra.RangeArgs(1, 3), RunE: func(cmd *cobra.Command, args []string) error {
		pageID, err := parseID(args[0])
		if err != nil {
			return err
		}
		client, err := a.getClient()
		if err != nil {
			return err
		}
		result, err := buildDiff(cmd.Context(), client, pageID, args[1:])
		if err != nil {
			return err
		}
		if a.format == "json" {
			return output.JSON(a.out, result)
		}
		if result.Diff == "" {
			_, err = fmt.Fprintf(a.out, "No differences between %s and %s\n", result.From, result.To)
			return err
		}
		_, err = fmt.Fprint(a.out, result.Diff)
		return err
	}}
}

func buildDiff(ctx context.Context, client WikiClient, pageID int, versions []string) (diffResult, error) {
	var fromLabel, toLabel, fromContent, toContent, fromTitle, toTitle, fromAuthor, toAuthor string
	switch len(versions) {
	case 0:
		history, err := client.PageVersions(ctx, pageID)
		if err != nil {
			return diffResult{}, err
		}
		sort.SliceStable(history, func(i, j int) bool {
			return history[i].VersionID > history[j].VersionID
		})
		if len(history) < 2 {
			return diffResult{}, errors.New("at least two versions are required for default diff")
		}
		fromVersion := history[1].VersionID
		toVersion := history[0].VersionID
		from, err := client.GetPageVersion(ctx, pageID, fromVersion)
		if err != nil {
			return diffResult{}, err
		}
		to, err := client.GetPageVersion(ctx, pageID, toVersion)
		if err != nil {
			return diffResult{}, err
		}
		fromLabel, toLabel = versionLabel(fromVersion), versionLabel(toVersion)
		fromContent, toContent = from.Content, to.Content
		fromTitle, toTitle = from.Title, to.Title
		fromAuthor, toAuthor = from.AuthorName, to.AuthorName
	case 1:
		fromVersion, err := parseID(versions[0])
		if err != nil {
			return diffResult{}, err
		}
		from, err := client.GetPageVersion(ctx, pageID, fromVersion)
		if err != nil {
			return diffResult{}, err
		}
		to, err := client.GetPage(ctx, strconv.Itoa(pageID), "", false)
		if err != nil {
			return diffResult{}, err
		}
		fromLabel, toLabel = versionLabel(fromVersion), "current"
		fromContent, toContent = from.Content, to.Content
		fromTitle, toTitle = from.Title, to.Title
		fromAuthor, toAuthor = from.AuthorName, to.AuthorName
	case 2:
		fromVersion, err := parseID(versions[0])
		if err != nil {
			return diffResult{}, err
		}
		toVersion, err := parseID(versions[1])
		if err != nil {
			return diffResult{}, err
		}
		from, err := client.GetPageVersion(ctx, pageID, fromVersion)
		if err != nil {
			return diffResult{}, err
		}
		to, err := client.GetPageVersion(ctx, pageID, toVersion)
		if err != nil {
			return diffResult{}, err
		}
		fromLabel, toLabel = versionLabel(fromVersion), versionLabel(toVersion)
		fromContent, toContent = from.Content, to.Content
		fromTitle, toTitle = from.Title, to.Title
		fromAuthor, toAuthor = from.AuthorName, to.AuthorName
	default:
		return diffResult{}, errors.New("too many version arguments")
	}
	diff := unifiedLineDiff(fromLabel, toLabel, fromContent, toContent)
	return diffResult{
		PageID: pageID, From: fromLabel, To: toLabel, Changed: diff != "", Diff: diff,
		FromTitle: fromTitle, ToTitle: toTitle, FromAuthor: fromAuthor, ToAuthor: toAuthor,
	}, nil
}

func versionLabel(versionID int) string {
	return "version " + strconv.Itoa(versionID)
}

func unifiedLineDiff(fromLabel, toLabel, from, to string) string {
	if from == to {
		return ""
	}
	fromLines := splitDiffLines(from)
	toLines := splitDiffLines(to)
	table := make([][]int, len(fromLines)+1)
	for i := range table {
		table[i] = make([]int, len(toLines)+1)
	}
	for i := len(fromLines) - 1; i >= 0; i-- {
		for j := len(toLines) - 1; j >= 0; j-- {
			if fromLines[i] == toLines[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else if table[i+1][j] >= table[i][j+1] {
				table[i][j] = table[i+1][j]
			} else {
				table[i][j] = table[i][j+1]
			}
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n+++ %s\n", fromLabel, toLabel)
	i, j := 0, 0
	for i < len(fromLines) && j < len(toLines) {
		if fromLines[i] == toLines[j] {
			fmt.Fprintf(&b, " %s\n", fromLines[i])
			i++
			j++
		} else if table[i+1][j] >= table[i][j+1] {
			fmt.Fprintf(&b, "-%s\n", fromLines[i])
			i++
		} else {
			fmt.Fprintf(&b, "+%s\n", toLines[j])
			j++
		}
	}
	for i < len(fromLines) {
		fmt.Fprintf(&b, "-%s\n", fromLines[i])
		i++
	}
	for j < len(toLines) {
		fmt.Fprintf(&b, "+%s\n", toLines[j])
		j++
	}
	return b.String()
}

func splitDiffLines(content string) []string {
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}
