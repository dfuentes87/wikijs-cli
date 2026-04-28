package tree

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hopyky/wikijs-cli/internal/api"
)

type node struct {
	children map[string]*node
	pages    []api.Page
}

func Render(pages []api.Page) string {
	root := &node{children: map[string]*node{}}
	for _, page := range pages {
		current := root
		parts := strings.FieldsFunc(strings.Trim(page.Path, "/"), func(r rune) bool { return r == '/' })
		for _, part := range parts {
			if current.children[part] == nil {
				current.children[part] = &node{children: map[string]*node{}}
			}
			current = current.children[part]
		}
		current.pages = append(current.pages, page)
	}
	var lines []string
	renderNode(root, "", "", true, &lines)
	return strings.Join(lines, "\n")
}

func renderNode(n *node, name, prefix string, last bool, lines *[]string) {
	if name != "" {
		connector := "|-- "
		if last {
			connector = "`-- "
		}
		*lines = append(*lines, prefix+connector+name+"/")
		if last {
			prefix += "    "
		} else {
			prefix += "|   "
		}
	}
	sort.Slice(n.pages, func(i, j int) bool { return n.pages[i].Path < n.pages[j].Path })
	childNames := make([]string, 0, len(n.children))
	for child := range n.children {
		childNames = append(childNames, child)
	}
	sort.Strings(childNames)
	total := len(n.pages) + len(childNames)
	for i, page := range n.pages {
		connector := "|-- "
		if i == total-1 {
			connector = "`-- "
		}
		*lines = append(*lines, fmt.Sprintf("%s%s%s (%d)", prefix, connector, page.Title, page.ID))
	}
	for i, child := range childNames {
		renderNode(n.children[child], child, prefix, i+len(n.pages) == total-1, lines)
	}
}
