package tree

import (
	"strings"
	"testing"

	"github.com/dfuentes87/wikijs-cli/internal/api"
)

func TestRender(t *testing.T) {
	got := Render([]api.Page{
		{ID: 2, Path: "docs/guide", Title: "Guide"},
		{ID: 1, Path: "docs", Title: "Docs"},
	})
	for _, want := range []string{"docs/", "Docs (1)", "Guide (2)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("tree missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "guide/") {
		t.Fatalf("tree rendered leaf page as folder:\n%s", got)
	}
}

func TestRenderSingleSegmentPageAsLeaf(t *testing.T) {
	got := Render([]api.Page{
		{ID: 1, Path: "home", Title: "home"},
		{ID: 2, Path: "new-dir/subpage", Title: "sub Page"},
	})
	for _, want := range []string{"|-- home (1)", "`-- sub Page (2)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("tree missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "home/") || strings.Contains(got, "subpage/") {
		t.Fatalf("tree rendered page-only path segments as folders:\n%s", got)
	}
}
