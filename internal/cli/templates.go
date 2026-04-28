package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dfuentes87/wikijs-cli/internal/output"
)

func (a *app) templateCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "template", Short: "Manage reusable page templates"}
	cmd.AddCommand(a.templateListCommand(), a.templateCreateCommand(), a.templateShowCommand(), a.templateDeleteCommand())
	return cmd
}

func (a *app) templateListCommand() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List templates", RunE: func(cmd *cobra.Command, args []string) error {
		names, err := listTemplates()
		if err != nil {
			return err
		}
		rows := make([][]string, 0, len(names))
		for _, name := range names {
			rows = append(rows, []string{name})
		}
		return a.print(names, []string{"Name"}, rows)
	}}
}

func (a *app) templateCreateCommand() *cobra.Command {
	var file, content string
	var stdin bool
	cmd := &cobra.Command{Use: "create <name>", Short: "Create or replace a template", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		body, err := readContent(a.in, file, content, stdin)
		if err != nil {
			return err
		}
		if body == "" {
			return errors.New("template content is required")
		}
		if err := saveTemplate(args[0], body); err != nil {
			return err
		}
		if a.format == "json" {
			return output.JSON(a.out, successResult{Success: true, Action: "template-create", Path: args[0]})
		}
		_, err = fmt.Fprintf(a.out, "Saved template %s\n", args[0])
		return err
	}}
	cmd.Flags().StringVar(&file, "file", "", "read template from file")
	cmd.Flags().StringVar(&content, "content", "", "inline template content")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read template from stdin")
	return cmd
}

func (a *app) templateShowCommand() *cobra.Command {
	return &cobra.Command{Use: "show <name>", Short: "Show a template", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		content, err := loadTemplate(args[0])
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(a.out, content)
		return err
	}}
}

func (a *app) templateDeleteCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{Use: "delete <name>", Short: "Delete a template", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !force && !a.confirm(fmt.Sprintf("Delete template %s?", args[0])) {
			return errors.New("delete cancelled")
		}
		if err := deleteTemplate(args[0]); err != nil {
			return err
		}
		if a.format == "json" {
			return output.JSON(a.out, successResult{Success: true, Action: "template-delete", Path: args[0]})
		}
		_, err := fmt.Fprintf(a.out, "Deleted template %s\n", args[0])
		return err
	}}
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")
	return cmd
}

func (a *app) renderTemplate(name string, values map[string]string) (string, error) {
	content, err := loadTemplate(name)
	if err != nil {
		return "", err
	}
	for key, value := range values {
		content = strings.ReplaceAll(content, "{{"+key+"}}", value)
	}
	return content, nil
}

func templateDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "wikijs", "templates"), nil
	}
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "wikijs", "templates"), nil
}

func templatePath(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, `/\`) {
		return "", errors.New("template name must not contain path separators")
	}
	dir, err := templateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".md"), nil
}

func listTemplates() ([]string, error) {
	dir, err := templateDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), ".md"))
	}
	sort.Strings(names)
	return names, nil
}

func loadTemplate(name string) (string, error) {
	path, err := templatePath(name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	return string(data), err
}

func saveTemplate(name, content string) error {
	path, err := templatePath(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func deleteTemplate(name string) error {
	path, err := templatePath(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}
