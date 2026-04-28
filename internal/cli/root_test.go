package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dfuentes87/wikijs-cli/internal/api"
)

type fakeClient struct{}

func (fakeClient) ListPages(context.Context, api.ListOptions) ([]api.Page, error) {
	return []api.Page{{ID: 1, Path: "home", Title: "Home", Locale: "en", IsPublished: true}}, nil
}
func (fakeClient) SearchPages(context.Context, string, int) (api.SearchResult, error) {
	return api.SearchResult{Results: []api.Page{{ID: 1, Path: "home", Title: "Home", Locale: "en"}}, TotalHits: 1}, nil
}
func (fakeClient) GetPage(context.Context, string, string, bool) (api.Page, error) {
	return api.Page{ID: 1, Path: "home", Title: "Home", Locale: "en", Content: "# Home", IsPublished: true}, nil
}
func (fakeClient) CreatePage(context.Context, api.CreatePageInput) (api.Page, error) {
	return api.Page{ID: 2, Path: "new", Title: "New"}, nil
}
func (fakeClient) UpdatePage(context.Context, api.UpdatePageInput) (api.Page, error) {
	return api.Page{ID: 1, Path: "home", Title: "Home"}, nil
}
func (fakeClient) MovePage(context.Context, int, string, string) error { return nil }
func (fakeClient) DeletePage(context.Context, int) error               { return nil }
func (fakeClient) ListTags(context.Context) ([]api.Tag, error) {
	return []api.Tag{{ID: 1, Tag: "docs", Title: "Docs"}}, nil
}
func (fakeClient) ListAssets(context.Context, string, int) ([]api.Asset, error) {
	return []api.Asset{{ID: 1, Filename: "image.png", Kind: "IMAGE", FileSize: 12}}, nil
}
func (fakeClient) UploadAsset(context.Context, string, string) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
}
func (fakeClient) DeleteAsset(context.Context, int) error { return nil }
func (fakeClient) Health(context.Context) (api.SystemInfo, error) {
	return api.SystemInfo{CurrentVersion: "2.5.0", LatestVersion: "2.5.0", Hostname: "wiki", Platform: "linux"}, nil
}
func (fakeClient) Stats(context.Context) (api.Stats, error) {
	return api.Stats{TotalPages: 1, PublishedPages: 1, Locales: map[string]int{"en": 1}, TopTags: map[string]int{"docs": 1}}, nil
}
func (fakeClient) PageVersions(context.Context, int) ([]api.Version, error) {
	return []api.Version{{VersionID: 2, AuthorName: "Admin", ActionType: "updated"}, {VersionID: 1, AuthorName: "Admin", ActionType: "created"}}, nil
}
func (fakeClient) GetPageVersion(context.Context, int, int) (api.PageVersion, error) {
	return api.PageVersion{VersionID: 1, Title: "Home", Content: "# Old Home"}, nil
}
func (fakeClient) RevertPage(context.Context, int, int) error { return nil }

func TestGetRaw(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: fakeClient{}})
	cmd.SetArgs([]string{"get", "1", "--raw"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "# Home" {
		t.Fatalf("out = %q", out.String())
	}
}

func TestDeleteRequiresConfirmation(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader("no\n"), client: fakeClient{}})
	cmd.SetArgs([]string{"delete", "1"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestListJSON(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: fakeClient{}})
	cmd.SetArgs([]string{"--format", "json", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"title": "Home"`) {
		t.Fatalf("out = %s", out.String())
	}
}

func TestSuccessCommandsEmitJSON(t *testing.T) {
	tests := [][]string{
		{"--format", "json", "move", "1", "/new/path"},
		{"--format", "json", "delete", "1", "--force"},
		{"--format", "json", "revert", "1", "2", "--force"},
		{"--format", "json", "asset", "upload", "asset.txt"},
		{"--format", "json", "asset", "delete", "1", "--force"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, errOut bytes.Buffer
			cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: fakeClient{}})
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var body struct {
				Success bool   `json:"success"`
				Action  string `json:"action"`
			}
			if err := json.Unmarshal(out.Bytes(), &body); err != nil {
				t.Fatalf("invalid json %q: %v", out.String(), err)
			}
			if !body.Success || body.Action == "" {
				t.Fatalf("unexpected success body: %+v", body)
			}
		})
	}
}

func TestVersionJSON(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: fakeClient{}})
	cmd.SetArgs([]string{"--format", "json", "version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var body map[string]string
	if err := json.Unmarshal(out.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["version"] == "" {
		t.Fatalf("version missing: %+v", body)
	}
}

func TestCompletionCommandGeneratesScriptWithoutConfig(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader("")})
	cmd.SetArgs([]string{"completion", "bash"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "bash completion") || !strings.Contains(out.String(), "wikijs") {
		t.Fatalf("unexpected completion output: %s", out.String())
	}
}

func TestShellRunsCommands(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader("version\nexit\n"), client: fakeClient{}})
	cmd.SetArgs([]string{"shell"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "wikijs dev") {
		t.Fatalf("shell output = %q", out.String())
	}
}

func TestAssetCommandIsSingular(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: fakeClient{}})
	cmd.SetArgs([]string{"asset", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "image.png") {
		t.Fatalf("asset output = %q", out.String())
	}

	out.Reset()
	errOut.Reset()
	cmd = newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: fakeClient{}})
	cmd.SetArgs([]string{"asset" + "s", "list"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected plural asset command to be unavailable")
	}
}

func TestExportWritesMarkdownAndJSON(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: fakeClient{}})
	cmd.SetArgs([]string{"export", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "home.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Home" {
		t.Fatalf("markdown export = %q", string(data))
	}

	jsonDir := t.TempDir()
	out.Reset()
	errOut.Reset()
	cmd = newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: fakeClient{}})
	cmd.SetArgs([]string{"--format", "json", "export", jsonDir, "--file-format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(jsonDir, "home.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"title": "Home"`) || !strings.Contains(out.String(), `"action": "export"`) {
		t.Fatalf("json export file=%s out=%s", string(data), out.String())
	}
}

type brokenContentClient struct {
	fakeClient
	content string
	assets  []api.Asset
}

func (c brokenContentClient) GetPage(context.Context, string, string, bool) (api.Page, error) {
	return api.Page{ID: 1, Path: "home", Title: "Home", Locale: "en", Content: c.content, IsPublished: true}, nil
}

func (c brokenContentClient) ListAssets(context.Context, string, int) ([]api.Asset, error) {
	return c.assets, nil
}

func TestCheckLinksReportsBrokenInternalLinks(t *testing.T) {
	var out, errOut bytes.Buffer
	client := brokenContentClient{content: "# Home\n[Missing](/missing)\n[External](https://example.com)"}
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: client})
	cmd.SetArgs([]string{"check-links"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "broken internal links") {
		t.Fatalf("expected broken links error, got %v", err)
	}
	if !strings.Contains(out.String(), "/missing") {
		t.Fatalf("check-links output = %q", out.String())
	}
}

type diffClient struct{ fakeClient }

func (diffClient) GetPageVersion(_ context.Context, _ int, versionID int) (api.PageVersion, error) {
	content := "# Old"
	if versionID == 2 {
		content = "# New"
	}
	return api.PageVersion{VersionID: versionID, Title: "Home", Content: content}, nil
}

func TestDiffComparesVersions(t *testing.T) {
	tests := [][]string{
		{"diff", "1"},
		{"diff", "1", "1"},
		{"diff", "1", "1", "2"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, errOut bytes.Buffer
			cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: diffClient{}})
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "---") || !strings.Contains(out.String(), "+#") {
				t.Fatalf("diff output = %q", out.String())
			}
		})
	}
}

type cloneClient struct {
	fakeClient
	input api.CreatePageInput
}

func (c *cloneClient) GetPage(context.Context, string, string, bool) (api.Page, error) {
	return api.Page{ID: 1, Path: "home", Title: "Home", Description: "desc", Locale: "en", Content: "# Home", Tags: api.Tags{"docs"}, IsPublished: true}, nil
}

func (c *cloneClient) CreatePage(_ context.Context, input api.CreatePageInput) (api.Page, error) {
	c.input = input
	return api.Page{ID: 2, Path: input.Path, Title: input.Title}, nil
}

func TestCloneCopiesContentAndTagsWhenRequested(t *testing.T) {
	var out, errOut bytes.Buffer
	client := &cloneClient{}
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: client})
	cmd.SetArgs([]string{"clone", "1", "/copy", "--with-tags"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if client.input.Path != "/copy" || client.input.Content != "# Home" || len(client.input.Tags) != 1 || client.input.Tags[0] != "docs" {
		t.Fatalf("clone input = %+v", client.input)
	}
}

func TestValidateReportsContentProblems(t *testing.T) {
	var out, errOut bytes.Buffer
	client := brokenContentClient{content: "#Bad\n[Missing](/missing)\n![Missing](/missing.png)"}
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: client})
	cmd.SetArgs([]string{"validate", "1"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("expected validation error, got %v", err)
	}
	if !strings.Contains(out.String(), "heading-space") || !strings.Contains(out.String(), "broken link") || !strings.Contains(out.String(), "broken image") {
		t.Fatalf("validate output = %q", out.String())
	}
}
