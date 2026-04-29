package cli

import (
	"bytes"
	"strings"
	"testing"
)

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

func TestTemplateShowIsUncolored(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := saveTemplate("doc", "# {{title}}"); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader("")})
	cmd.SetArgs([]string{"template", "show", "doc"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if hasANSI(out.String()) || out.String() != "# {{title}}" {
		t.Fatalf("template show output = %q", out.String())
	}
}
