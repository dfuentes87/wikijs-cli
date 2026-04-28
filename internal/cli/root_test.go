package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
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

func TestVersionCommandUnavailable(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: fakeClient{}})
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected version command to be unavailable")
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
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader("list\nexit\n"), client: fakeClient{}})
	cmd.SetArgs([]string{"shell"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Home") {
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

type multiPageClient struct {
	fakeClient
	pages []api.Page
}

func (c multiPageClient) ListPages(context.Context, api.ListOptions) ([]api.Page, error) {
	return c.pages, nil
}

func (c multiPageClient) GetPage(_ context.Context, idOrPath string, _ string, _ bool) (api.Page, error) {
	for _, page := range c.pages {
		if idOrPath == strconvItoa(page.ID) || idOrPath == page.Path {
			return page, nil
		}
	}
	return api.Page{}, api.ErrNotFound
}

func TestSyncWritesMarkdownAndSkipsUnchanged(t *testing.T) {
	dir := t.TempDir()
	client := multiPageClient{pages: []api.Page{{ID: 1, Path: "home", Title: "Home", Locale: "en", Content: "# Home", IsPublished: true}}}

	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: client})
	cmd.SetArgs([]string{"sync", "--output", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "home.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Home" || !strings.Contains(out.String(), "1 created") {
		t.Fatalf("first sync data=%q out=%q", string(data), out.String())
	}

	out.Reset()
	errOut.Reset()
	cmd = newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: client})
	cmd.SetArgs([]string{"sync", "--output", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "1 skipped") {
		t.Fatalf("second sync output = %q", out.String())
	}
}

func TestSyncUsesConfigPathWhenOutputOmitted(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "wikijs.json")
	configBody := `{"url":"https://wiki.example.test","apiToken":"token","autoSync":{"path":` + strconv.Quote(dir) + `}}`
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: fakeClient{}})
	cmd.SetArgs([]string{"--config", configPath, "sync"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "home.md")); err != nil {
		t.Fatal(err)
	}
}

func TestSyncJSONOutput(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: fakeClient{}})
	cmd.SetArgs([]string{"--format", "json", "sync", "--output", t.TempDir(), "--file-format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Success bool   `json:"success"`
		Action  string `json:"action"`
		Result  struct {
			Created int `json:"created"`
			Pages   int `json:"pages"`
			Files   int `json:"files"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Success || body.Action != "sync" || body.Result.Created != 1 || body.Result.Pages != 1 || body.Result.Files != 1 {
		t.Fatalf("unexpected sync json: %+v", body)
	}
}

func TestSyncDeletesStaleFilesForSelectedFormat(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stale.md"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "keep.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: fakeClient{}})
	cmd.SetArgs([]string{"sync", "--output", dir, "--delete"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "stale.md")); !os.IsNotExist(err) {
		t.Fatalf("stale markdown file was not deleted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.json")); err != nil {
		t.Fatalf("json file should not be deleted during markdown sync: %v", err)
	}
}

func TestSyncPathFilter(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "home.md"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := multiPageClient{pages: []api.Page{
		{ID: 1, Path: "home", Title: "Home", Locale: "en", Content: "# Home", IsPublished: true},
		{ID: 2, Path: "docs/guide", Title: "Guide", Locale: "en", Content: "# Guide", IsPublished: true},
	}}
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: client})
	cmd.SetArgs([]string{"sync", "--output", dir, "--path", "/docs", "--delete"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "guide.md")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "home.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep" {
		t.Fatalf("home should not have been touched by scoped sync: %q", string(data))
	}
}

type tagClient struct {
	fakeClient
	input api.UpdatePageInput
}

func (c *tagClient) GetPage(context.Context, string, string, bool) (api.Page, error) {
	return api.Page{ID: 1, Path: "home", Title: "Home", Locale: "en", Content: "# Home", Tags: api.Tags{"docs"}, IsPublished: true}, nil
}

func (c *tagClient) UpdatePage(_ context.Context, input api.UpdatePageInput) (api.Page, error) {
	c.input = input
	return api.Page{ID: input.ID, Path: "home", Title: "Home", Tags: api.Tags(input.Tags)}, nil
}

func TestTagCommandsUpdateTags(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{[]string{"tag", "1", "add", "new"}, []string{"docs", "new"}},
		{[]string{"tag", "1", "remove", "docs"}, nil},
		{[]string{"tag", "1", "set", "a,b"}, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var out, errOut bytes.Buffer
			client := &tagClient{}
			cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: client})
			cmd.SetArgs(tt.args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !client.input.SetTags || strings.Join(client.input.Tags, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("input = %+v, want tags %v", client.input, tt.want)
			}
		})
	}
}

func TestGrepFindsPageContent(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: fakeClient{}})
	cmd.SetArgs([]string{"grep", "home"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "home") || !strings.Contains(out.String(), "# Home") {
		t.Fatalf("grep output = %q", out.String())
	}
}

func TestInfoShowsMetadata(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: fakeClient{}})
	cmd.SetArgs([]string{"info", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Path") || !strings.Contains(out.String(), "home") {
		t.Fatalf("info output = %q", out.String())
	}
}

func TestStatsDetailedIncludesContentMetrics(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&app{format: "table", out: &out, errOut: &errOut, in: strings.NewReader(""), client: fakeClient{}})
	cmd.SetArgs([]string{"stats", "--detailed"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Words") || !strings.Contains(out.String(), "Estimated read minutes") {
		t.Fatalf("stats output = %q", out.String())
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
	if !strings.HasSuffix(errOut.String(), "\n\n") {
		t.Fatalf("check-links error output = %q, want blank line", errOut.String())
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
