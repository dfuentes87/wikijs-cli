package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/dfuentes87/wikijs-cli/internal/config"
)

func TestListPagesSendsAuthAndDecodes(t *testing.T) {
	var sawAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer token" {
			sawAuth = true
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"pages": map[string]any{"list": []map[string]any{
				{"id": 1, "path": "home", "title": "Home", "locale": "en", "tags": []string{"docs"}, "isPublished": true},
			}}},
		})
	}))
	defer server.Close()

	client := New(config.Config{URL: server.URL, APIToken: "token", DefaultLocale: "en", DefaultEditor: "markdown"})
	pages, err := client.ListPages(context.Background(), ListOptions{Tag: "docs"})
	if err != nil {
		t.Fatal(err)
	}
	if !sawAuth {
		t.Fatal("auth header not sent")
	}
	if len(pages) != 1 || pages[0].Title != "Home" {
		t.Fatalf("unexpected pages: %+v", pages)
	}
}

func TestGraphQLError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"errors": []map[string]string{{"message": "boom"}}})
	}))
	defer server.Close()

	client := New(config.Config{URL: server.URL, APIToken: "token", DefaultLocale: "en", DefaultEditor: "markdown"})
	_, err := client.Health(context.Background())
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected graphql error, got %v", err)
	}
}

func TestAuthenticationError(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, http.StatusText(status), status)
			}))
			defer server.Close()

			client := New(config.Config{URL: server.URL, APIToken: "token", DefaultLocale: "en", DefaultEditor: "markdown"})
			_, err := client.Health(context.Background())
			var authErr AuthError
			if !errors.As(err, &authErr) {
				t.Fatalf("expected AuthError, got %T %v", err, err)
			}
			if authErr.Status == "" {
				t.Fatal("expected status on auth error")
			}
		})
	}
}

func TestUploadAsset(t *testing.T) {
	var sawMultipart bool
	var sawMetadata bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/u" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			sawMultipart = true
		}
		if err := r.ParseMultipartForm(1024 * 1024); err != nil {
			t.Fatal(err)
		}
		if values := r.MultipartForm.Value["mediaUpload"]; len(values) > 0 && strings.Contains(values[0], `"folderId":0`) {
			sawMetadata = true
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()

	file := t.TempDir() + "/asset.txt"
	if err := os.WriteFile(file, []byte("asset"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := New(config.Config{URL: server.URL, APIToken: "token", DefaultLocale: "en", DefaultEditor: "markdown"})
	result, err := client.UploadAsset(context.Background(), file, "")
	if err != nil {
		t.Fatal(err)
	}
	if !sawMultipart || !sawMetadata || result["ok"] != true {
		t.Fatalf("unexpected upload result: sawMultipart=%v sawMetadata=%v result=%v", sawMultipart, sawMetadata, result)
	}
}

func TestMutationResultDecodesNumericErrorCode(t *testing.T) {
	var result mutationResult
	if err := json.Unmarshal([]byte(`{"responseResult":{"succeeded":true,"errorCode":0,"message":"ok"},"page":{"id":1}}`), &result); err != nil {
		t.Fatal(err)
	}
	if result.ResponseResult.ErrorCode != "0" {
		t.Fatalf("errorCode = %q", result.ResponseResult.ErrorCode)
	}
}

func TestUpdatePageSendsEmptyTagsWhenClearingTags(t *testing.T) {
	var sawTags bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		page := map[string]any{"id": 1, "path": "home", "title": "Home", "content": "# Home", "tags": []map[string]string{}, "isPublished": true}
		if strings.Contains(req.Query, "single(id") {
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"pages": map[string]any{"single": page}}})
			return
		}
		tags, ok := req.Variables["tags"].([]any)
		if ok && len(tags) == 0 {
			sawTags = true
		}
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"pages": map[string]any{"update": map[string]any{"responseResult": map[string]any{"succeeded": true}, "page": page}}}})
	}))
	defer server.Close()

	client := New(config.Config{URL: server.URL, APIToken: "token", DefaultLocale: "en", DefaultEditor: "markdown"})
	if _, err := client.UpdatePage(context.Background(), UpdatePageInput{ID: 1, SetTags: true}); err != nil {
		t.Fatal(err)
	}
	if !sawTags {
		t.Fatal("expected empty tags slice in update variables")
	}
}

func TestSearchPagesDecodesStringIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"pages": map[string]any{"search": map[string]any{
				"results": []map[string]any{{"id": "42", "path": "home", "title": "Home", "locale": "en"}},
			}}},
		})
	}))
	defer server.Close()

	client := New(config.Config{URL: server.URL, APIToken: "token", DefaultLocale: "en", DefaultEditor: "markdown"})
	result, err := client.SearchPages(context.Background(), "home", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 || result.Results[0].ID != 42 {
		t.Fatalf("unexpected search results: %+v", result.Results)
	}
}

func TestListAssetsIncludesSubfolders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables map[string]int `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		switch req.Variables["folderID"] {
		case 0:
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"assets": map[string]any{
				"list":    []map[string]any{{"id": 1, "filename": "root.png", "kind": "IMAGE", "fileSize": 1}},
				"folders": []map[string]any{{"id": 2, "slug": "uploads", "name": "Uploads"}},
			}}})
		case 2:
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"assets": map[string]any{
				"list":    []map[string]any{{"id": 3, "filename": "nested.png", "kind": "IMAGE", "fileSize": 2}},
				"folders": []map[string]any{{"id": 4, "slug": "icons", "name": "Icons"}},
			}}})
		case 4:
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"assets": map[string]any{
				"list":    []map[string]any{{"id": 2, "filename": "deep.png", "kind": "IMAGE", "fileSize": 3}},
				"folders": []map[string]any{},
			}}})
		default:
			t.Fatalf("unexpected folderID %d", req.Variables["folderID"])
		}
	}))
	defer server.Close()

	client := New(config.Config{URL: server.URL, APIToken: "token", DefaultLocale: "en", DefaultEditor: "markdown"})
	assets, err := client.ListAssets(context.Background(), "", 0)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(assets))
	for _, asset := range assets {
		got = append(got, asset.Filename)
	}
	want := []string{"root.png", "uploads/icons/deep.png", "uploads/nested.png"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("asset filenames = %+v, want %+v", got, want)
	}

	filtered, err := client.ListAssets(context.Background(), "/uploads/icons", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].Filename != "uploads/icons/deep.png" {
		t.Fatalf("filtered assets = %+v", filtered)
	}
}

func TestGraphQLOperationsUseVariables(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, req)
		query := req["query"].(string)
		json.NewEncoder(w).Encode(graphQLResponseFor(query))
	}))
	defer server.Close()

	client := New(config.Config{URL: server.URL, APIToken: "token", DefaultLocale: "en", DefaultEditor: "markdown"})
	ctx := context.Background()
	must(client.Health(ctx))(t)
	must(client.ListPages(ctx, ListOptions{Tag: "docs", Locale: "en"}))(t)
	must(client.SearchPages(ctx, "home", 10))(t)
	must(client.GetPage(ctx, "1", "", false))(t)
	must(client.GetPage(ctx, "/home", "en", false))(t)
	must(client.CreatePage(ctx, CreatePageInput{Path: "/new", Title: "New", Tags: []string{"docs"}, IsPublished: true}))(t)
	must(client.UpdatePage(ctx, UpdatePageInput{ID: 1, Title: stringPtr("Updated")}))(t)
	mustErr(client.MovePage(ctx, 1, "/moved", "en"))(t)
	mustErr(client.DeletePage(ctx, 1))(t)
	must(client.ListTags(ctx))(t)
	must(client.ListAssets(ctx, "", 10))(t)
	mustErr(client.DeleteAsset(ctx, 7))(t)
	must(client.PageVersions(ctx, 1))(t)
	must(client.GetPageVersion(ctx, 1, 2))(t)
	mustErr(client.RevertPage(ctx, 1, 2))(t)

	assertRequestVariables(t, requests, "search(query: $query)", map[string]any{"query": "home"})
	assertRequestVariables(t, requests, "singleByPath", map[string]any{"path": "home", "locale": "en"})
	assertRequestVariables(t, requests, "create(content", map[string]any{"path": "new", "title": "New"})
	assertRequestVariables(t, requests, "update(id", map[string]any{"id": float64(1), "title": "Updated"})
	assertRequestVariables(t, requests, "destinationPath", map[string]any{"id": float64(1), "path": "moved"})
	assertRequestVariables(t, requests, "delete(id", map[string]any{"id": float64(1)})
	assertRequestVariables(t, requests, "deleteAsset", map[string]any{"id": float64(7)})
	assertRequestVariables(t, requests, "history(id", map[string]any{"id": float64(1)})
	assertRequestVariables(t, requests, "version(pageId", map[string]any{"pageID": float64(1), "versionID": float64(2)})
	assertRequestVariables(t, requests, "restore(pageId", map[string]any{"pageID": float64(1), "versionID": float64(2)})
}

func graphQLResponseFor(query string) map[string]any {
	page := map[string]any{"id": 1, "path": "home", "title": "Home", "locale": "en", "content": "# Home", "tags": []map[string]string{{"tag": "docs"}}, "isPublished": true}
	responseResult := map[string]any{"responseResult": map[string]any{"succeeded": true}, "page": page}
	switch {
	case strings.Contains(query, "system"):
		return map[string]any{"data": map[string]any{"system": map[string]any{"info": map[string]any{"currentVersion": "2.5.0"}}}}
	case strings.Contains(query, "pages { list"):
		return map[string]any{"data": map[string]any{"pages": map[string]any{"list": []map[string]any{page}}}}
	case strings.Contains(query, "search(query"):
		return map[string]any{"data": map[string]any{"pages": map[string]any{"search": map[string]any{"results": []map[string]any{page}, "totalHits": 1}}}}
	case strings.Contains(query, "single(id"):
		return map[string]any{"data": map[string]any{"pages": map[string]any{"single": page}}}
	case strings.Contains(query, "singleByPath"):
		return map[string]any{"data": map[string]any{"pages": map[string]any{"singleByPath": page}}}
	case strings.Contains(query, "create("):
		return map[string]any{"data": map[string]any{"pages": map[string]any{"create": responseResult}}}
	case strings.Contains(query, "update("):
		return map[string]any{"data": map[string]any{"pages": map[string]any{"update": responseResult}}}
	case strings.Contains(query, "move("):
		return map[string]any{"data": map[string]any{"pages": map[string]any{"move": responseResult}}}
	case strings.Contains(query, "delete(id"):
		return map[string]any{"data": map[string]any{"pages": map[string]any{"delete": responseResult}}}
	case strings.Contains(query, "pages { tags"):
		return map[string]any{"data": map[string]any{"pages": map[string]any{"tags": []map[string]any{{"id": 1, "tag": "docs"}}}}}
	case strings.Contains(query, "assets { list"):
		return map[string]any{"data": map[string]any{"assets": map[string]any{"list": []map[string]any{{"id": 1, "filename": "image.png"}}}}}
	case strings.Contains(query, "deleteAsset"):
		return map[string]any{"data": map[string]any{"assets": map[string]any{"deleteAsset": responseResult}}}
	case strings.Contains(query, "history(id"):
		return map[string]any{"data": map[string]any{"pages": map[string]any{"history": map[string]any{"trail": []map[string]any{{"versionId": 1, "versionDate": "2026-01-01T00:00:00Z"}}, "total": 1}}}}
	case strings.Contains(query, "version(pageId"):
		return map[string]any{"data": map[string]any{"pages": map[string]any{"version": map[string]any{"versionId": 2, "content": "# Old", "title": "Home", "actionType": "updated"}}}}
	case strings.Contains(query, "restore("):
		return map[string]any{"data": map[string]any{"pages": map[string]any{"restore": responseResult}}}
	default:
		return map[string]any{"errors": []map[string]string{{"message": "unhandled query"}}}
	}
}

func assertRequestVariables(t *testing.T, requests []map[string]any, queryFragment string, want map[string]any) {
	t.Helper()
	for _, req := range requests {
		query, _ := req["query"].(string)
		if !strings.Contains(query, queryFragment) {
			continue
		}
		variables, _ := req["variables"].(map[string]any)
		for key, wantValue := range want {
			if variables[key] != wantValue {
				t.Fatalf("%s variable %s = %#v, want %#v in %#v", queryFragment, key, variables[key], wantValue, variables)
			}
		}
		return
	}
	t.Fatalf("request containing %q not found in %#v", queryFragment, requests)
}

func must[T any](value T, err error) func(*testing.T) {
	return func(t *testing.T) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func mustErr(err error) func(*testing.T) {
	return func(t *testing.T) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func stringPtr(value string) *string {
	return &value
}
