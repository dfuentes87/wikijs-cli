package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hopyky/wikijs-cli/internal/api"
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
	return []api.Version{{VersionID: 1, AuthorName: "Admin", ActionType: "updated"}}, nil
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
		{"--format", "json", "assets", "upload", "asset.txt"},
		{"--format", "json", "assets", "delete", "1", "--force"},
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
