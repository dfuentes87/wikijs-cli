package markdown

import (
	"strings"
)

type Link struct {
	Line   int    `json:"line"`
	Text   string `json:"text,omitempty"`
	Target string `json:"target"`
	Image  bool   `json:"image,omitempty"`
}

func Links(content string) []Link {
	var links []Link
	inFence := false
	fenceMarker := ""
	lines := strings.Split(content, "\n")
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if !inFence {
				inFence = true
				fenceMarker = marker
			} else if marker == fenceMarker {
				inFence = false
				fenceMarker = ""
			}
			continue
		}
		if inFence {
			continue
		}
		links = append(links, scanLineLinks(line, idx+1)...)
	}
	return links
}

func scanLineLinks(line string, lineNum int) []Link {
	var links []Link
	for i := 0; i < len(line); i++ {
		image := false
		if line[i] == '!' && i+1 < len(line) && line[i+1] == '[' {
			image = true
			i++
		}
		if line[i] != '[' {
			continue
		}
		closeText := findClosingBracket(line, i+1)
		if closeText < 0 || closeText+1 >= len(line) || line[closeText+1] != '(' {
			continue
		}
		closeTarget := findClosingParen(line, closeText+2)
		if closeTarget < 0 {
			continue
		}
		target := cleanLinkTarget(line[closeText+2 : closeTarget])
		if target != "" {
			links = append(links, Link{Line: lineNum, Text: line[i+1 : closeText], Target: target, Image: image})
		}
		i = closeTarget
	}
	return links
}

func findClosingBracket(line string, start int) int {
	escaped := false
	for i := start; i < len(line); i++ {
		if escaped {
			escaped = false
			continue
		}
		if line[i] == '\\' {
			escaped = true
			continue
		}
		if line[i] == ']' {
			return i
		}
	}
	return -1
}

func findClosingParen(line string, start int) int {
	escaped := false
	depth := 0
	for i := start; i < len(line); i++ {
		if escaped {
			escaped = false
			continue
		}
		switch line[i] {
		case '\\':
			escaped = true
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
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
