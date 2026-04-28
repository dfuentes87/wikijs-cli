package cli

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func (a *app) shellCommand() *cobra.Command {
	return &cobra.Command{Use: "shell", Short: "Run an interactive wikijs shell", RunE: func(cmd *cobra.Command, args []string) error {
		scanner := bufio.NewScanner(a.in)
		for {
			fmt.Fprint(a.errOut, "wikijs> ")
			if !scanner.Scan() {
				return scanner.Err()
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if line == "exit" || line == "quit" {
				return nil
			}
			parts, err := splitCommandLine(line)
			if err != nil {
				fmt.Fprintln(a.errOut, FormatError(err))
				continue
			}
			child := newRootCommand(a)
			child.SetArgs(parts)
			if err := child.ExecuteContext(cmd.Context()); err != nil {
				fmt.Fprintln(a.errOut, FormatError(err))
			}
		}
	}}
}

func splitCommandLine(line string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	escaped := false
	for _, r := range line {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, errors.New("unterminated quote")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, nil
}
