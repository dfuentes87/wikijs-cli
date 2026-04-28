package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSplitCommandLine(t *testing.T) {
	got, err := splitCommandLine(`create "/docs/new page" "New Page" --tag docs`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"create", "/docs/new page", "New Page", "--tag", "docs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestTemplateLifecycle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := saveTemplate("doc", "# {{title}}\n{{path}}\n{{date}}"); err != nil {
		t.Fatal(err)
	}
	names, err := listTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "doc" {
		t.Fatalf("names = %#v", names)
	}
	rendered, err := (&app{}).renderTemplate("doc", map[string]string{"title": "Hello", "path": "docs/hello", "date": "2026-04-28"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "Hello") || !strings.Contains(rendered, "docs/hello") {
		t.Fatalf("rendered = %q", rendered)
	}
	if err := deleteTemplate("doc"); err != nil {
		t.Fatal(err)
	}
}

func TestMarkdownFilesAndPagePath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "guide"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "guide", "intro.md"), []byte("# Intro"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skip.txt"), []byte("skip"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := markdownFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("files = %#v", files)
	}
	path, err := pagePathFromFile(root, files[0], "/docs")
	if err != nil {
		t.Fatal(err)
	}
	if path != "docs/guide/intro" {
		t.Fatalf("path = %q", path)
	}
}

func TestReplacer(t *testing.T) {
	replace, err := newReplacer("old", "new", false, false)
	if err != nil {
		t.Fatal(err)
	}
	got, changed := replace("OLD value")
	if !changed || got != "new value" {
		t.Fatalf("got %q changed=%v", got, changed)
	}
}
