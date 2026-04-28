package cli

import (
	"os"
	"path/filepath"
	"testing"
)

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
