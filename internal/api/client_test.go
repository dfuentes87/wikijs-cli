package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/hopyky/wikijs-cli/internal/config"
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

func TestUploadAsset(t *testing.T) {
	var sawMultipart bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/u" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			sawMultipart = true
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
	if !sawMultipart || result["ok"] != true {
		t.Fatalf("unexpected upload result: sawMultipart=%v result=%v", sawMultipart, result)
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
	mustErr(client.RevertPage(ctx, 1, 2))(t)

	assertRequestVariables(t, requests, "search(query: $query)", map[string]any{"query": "home"})
	assertRequestVariables(t, requests, "singleByPath", map[string]any{"path": "home", "locale": "en"})
	assertRequestVariables(t, requests, "create(content", map[string]any{"path": "new", "title": "New"})
	assertRequestVariables(t, requests, "update(id", map[string]any{"id": float64(1), "title": "Updated"})
	assertRequestVariables(t, requests, "destinationPath", map[string]any{"id": float64(1), "path": "moved"})
	assertRequestVariables(t, requests, "delete(id", map[string]any{"id": float64(1)})
	assertRequestVariables(t, requests, "deleteAsset", map[string]any{"id": float64(7)})
	assertRequestVariables(t, requests, "history(id", map[string]any{"id": float64(1)})
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
		return map[string]any{"data": map[string]any{"pages": map[string]any{"history": []map[string]any{{"versionId": 1, "versionDate": "2026-01-01T00:00:00Z"}}}}}
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
