package markdown

import (
	"fmt"
	"strings"
)

type Issue struct {
	Line    int    `json:"line"`
	Type    string `json:"type"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

type LintResult struct {
	Valid    bool    `json:"valid"`
	Errors   []Issue `json:"errors"`
	Warnings []Issue `json:"warnings"`
	All      []Issue `json:"all"`
}

func Lint(content string) LintResult {
	var issues []Issue
	lines := strings.Split(content, "\n")
	for idx, line := range lines {
		lineNum := idx + 1
		if len(line) > 0 && (strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t")) {
			issues = append(issues, Issue{Line: lineNum, Type: "warning", Rule: "no-trailing-spaces", Message: "Trailing whitespace"})
		}
		if idx > 0 && line == "" && lines[idx-1] == "" {
			issues = append(issues, Issue{Line: lineNum, Type: "warning", Rule: "no-multiple-blanks", Message: "Multiple consecutive blank lines"})
		}
		if strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "```") {
			markers := 0
			for markers < len(line) && line[markers] == '#' {
				markers++
			}
			if markers > 0 && markers < len(line) && line[markers] != ' ' && line[markers] != '#' {
				issues = append(issues, Issue{Line: lineNum, Type: "error", Rule: "heading-space", Message: "Missing space after heading markers"})
			}
		}
		if len(line) > 120 && !strings.Contains(line, "http") {
			issues = append(issues, Issue{Line: lineNum, Type: "warning", Rule: "line-length", Message: fmt.Sprintf("Line too long (%d > 120 characters)", len(line))})
		}
		if strings.Contains(line, "\t") {
			issues = append(issues, Issue{Line: lineNum, Type: "warning", Rule: "no-tabs", Message: "Tab character found (prefer spaces)"})
		}
		if strings.Contains(line, "](") && strings.Count(line, "(") > strings.Count(line, ")") {
			issues = append(issues, Issue{Line: lineNum, Type: "error", Rule: "valid-link", Message: "Unclosed link syntax"})
		}
	}
	firstHeading := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			firstHeading = line
			break
		}
	}
	if firstHeading != "" && !strings.HasPrefix(firstHeading, "# ") {
		issues = append(issues, Issue{Line: 1, Type: "warning", Rule: "first-heading-h1", Message: "First heading should be H1"})
	}
	if strings.TrimSpace(content) == "" {
		issues = append(issues, Issue{Line: 1, Type: "error", Rule: "no-empty", Message: "Document is empty"})
	}

	result := LintResult{Valid: true, All: issues}
	for _, issue := range issues {
		if issue.Type == "error" {
			result.Valid = false
			result.Errors = append(result.Errors, issue)
		} else {
			result.Warnings = append(result.Warnings, issue)
		}
	}
	return result
}

func Format(result LintResult) string {
	var lines []string
	for _, issue := range result.Errors {
		lines = append(lines, fmt.Sprintf("Error line %d: %s (%s)", issue.Line, issue.Message, issue.Rule))
	}
	for _, issue := range result.Warnings {
		lines = append(lines, fmt.Sprintf("Warning line %d: %s (%s)", issue.Line, issue.Message, issue.Rule))
	}
	if len(lines) == 0 {
		return "No issues found"
	}
	return strings.Join(lines, "\n")
}
