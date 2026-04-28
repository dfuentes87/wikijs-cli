package cli

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/dfuentes87/wikijs-cli/internal/api"
)

func (a *app) cloneCommand() *cobra.Command {
	var withTags bool
	var title string
	var locale string
	cmd := &cobra.Command{Use: "clone <id-or-path> <new-path>", Short: "Clone a page", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		client, err := a.getClient()
		if err != nil {
			return err
		}
		source, err := client.GetPage(cmd.Context(), args[0], locale, false)
		if err != nil {
			return err
		}
		if title == "" {
			title = source.Title
		}
		tags := []string(nil)
		if withTags {
			tags = []string(source.Tags)
		}
		created, err := client.CreatePage(cmd.Context(), api.CreatePageInput{
			Path:        args[1],
			Title:       title,
			Content:     source.Content,
			Description: source.Description,
			Tags:        tags,
			Locale:      firstNonEmpty(locale, source.Locale),
			IsPublished: source.IsPublished,
			IsPrivate:   source.IsPrivate,
		})
		if err != nil {
			return err
		}
		return a.print(created, []string{"ID", "Path", "Title"}, [][]string{{strconv.Itoa(created.ID), created.Path, created.Title}})
	}}
	cmd.Flags().BoolVar(&withTags, "with-tags", false, "copy source page tags")
	cmd.Flags().StringVar(&title, "title", "", "title for the cloned page")
	cmd.Flags().StringVar(&locale, "locale", "", "source and destination locale")
	return cmd
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
