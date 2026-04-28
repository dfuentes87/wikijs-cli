package tree

import (
	"strings"
	"testing"

	"github.com/hopyky/wikijs-cli/internal/api"
)

func TestRender(t *testing.T) {
	got := Render([]api.Page{
		{ID: 2, Path: "docs/guide", Title: "Guide"},
		{ID: 1, Path: "docs", Title: "Docs"},
	})
	for _, want := range []string{"docs/", "Docs (1)", "guide/", "Guide (2)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("tree missing %q:\n%s", want, got)
		}
	}
}
