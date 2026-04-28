package markdown

import (
	"regexp"
	"strings"
)

type Link struct {
	Line   int    `json:"line"`
	Text   string `json:"text,omitempty"`
	Target string `json:"target"`
	Image  bool   `json:"image,omitempty"`
}

var markdownLinkPattern = regexp.MustCompile(`!?\[[^\]]*\]\([^)]+\)`)

func Links(content string) []Link {
	var links []Link
	lines := strings.Split(content, "\n")
	for idx, line := range lines {
		for _, match := range markdownLinkPattern.FindAllString(line, -1) {
			open := strings.Index(match, "[")
			close := strings.Index(match, "](")
			if open < 0 || close < 0 || len(match) < 3 {
				continue
			}
			target := strings.TrimSpace(match[close+2 : len(match)-1])
			if target == "" {
				continue
			}
			links = append(links, Link{
				Line:   idx + 1,
				Text:   match[open+1 : close],
				Target: cleanLinkTarget(target),
				Image:  strings.HasPrefix(match, "!"),
			})
		}
	}
	return links
}

func cleanLinkTarget(target string) string {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "<") {
		if end := strings.Index(target, ">"); end >= 0 {
			return strings.TrimSpace(target[1:end])
		}
	}
	if idx := strings.IndexAny(target, " \t"); idx >= 0 {
		return strings.TrimSpace(target[:idx])
	}
	return target
}
