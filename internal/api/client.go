package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dfuentes87/wikijs-cli/internal/config"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation failed")
	ErrAuth       = errors.New("authentication failed")
)

type AuthError struct {
	Status string
}

func (e AuthError) Error() string {
	if e.Status == "" {
		return ErrAuth.Error()
	}
	return ErrAuth.Error() + ": " + e.Status
}

func (e AuthError) Unwrap() error {
	return ErrAuth
}

type Client struct {
	baseURL       string
	apiToken      string
	defaultLocale string
	defaultEditor string
	httpClient    *http.Client
	rateLimit     time.Duration
	lastRequest   time.Time
	logger        func(string, ...any)
	debug         bool
}

type Option func(*Client)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

func WithRateLimit(rateLimit time.Duration) Option {
	return func(c *Client) {
		c.rateLimit = rateLimit
	}
}

func WithLogger(logger func(string, ...any), debug bool) Option {
	return func(c *Client) {
		c.logger = logger
		c.debug = debug
	}
}

func New(cfg config.Config, opts ...Option) *Client {
	c := &Client{
		baseURL:       strings.TrimRight(cfg.URL, "/"),
		apiToken:      cfg.APIToken,
		defaultLocale: cfg.DefaultLocale,
		defaultEditor: cfg.DefaultEditor,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type GraphQLError struct {
	Message string `json:"message"`
}

type GraphQLErrors []GraphQLError

func (e GraphQLErrors) Error() string {
	parts := make([]string, 0, len(e))
	for _, item := range e {
		parts = append(parts, item.Message)
	}
	return strings.Join(parts, "; ")
}

func (c *Client) graphql(ctx context.Context, query string, variables map[string]any, out any) error {
	if err := c.wait(ctx); err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/graphql", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	c.logf("POST /graphql")
	if c.debug {
		c.logf("GraphQL variables: %s", redactVariables(variables))
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	c.logf("POST /graphql -> %s", resp.Status)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return AuthError{Status: resp.Status}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("http %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors GraphQLErrors   `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if len(envelope.Errors) > 0 {
		return envelope.Errors
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	return json.Unmarshal(envelope.Data, out)
}

func (c *Client) wait(ctx context.Context) error {
	if c.rateLimit <= 0 || c.lastRequest.IsZero() {
		c.lastRequest = time.Now()
		return nil
	}
	elapsed := time.Since(c.lastRequest)
	if elapsed < c.rateLimit {
		timer := time.NewTimer(c.rateLimit - elapsed)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	c.lastRequest = time.Now()
	return nil
}

func (c *Client) ListPages(ctx context.Context, opts ListOptions) ([]Page, error) {
	const query = `query { pages { list { id path title description locale createdAt updatedAt tags isPublished } } }`
	var data struct {
		Pages struct {
			List []Page `json:"list"`
		} `json:"pages"`
	}
	if err := c.graphql(ctx, query, nil, &data); err != nil {
		return nil, err
	}
	pages := data.Pages.List
	filtered := pages[:0]
	for _, page := range pages {
		if opts.Locale != "" && page.Locale != opts.Locale {
			continue
		}
		if opts.Tag != "" && !containsString(page.Tags, opts.Tag) {
			continue
		}
		filtered = append(filtered, page)
	}
	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
	}
	return filtered, nil
}

type ListOptions struct {
	Tag    string
	Locale string
	Limit  int
}

func (c *Client) SearchPages(ctx context.Context, searchQuery string, limit int) (SearchResult, error) {
	const query = `query($query: String!) { pages { search(query: $query) { results { id path title description locale } suggestions totalHits } } }`
	var data struct {
		Pages struct {
			Search SearchResult `json:"search"`
		} `json:"pages"`
	}
	if err := c.graphql(ctx, query, map[string]any{"query": searchQuery}, &data); err != nil {
		return SearchResult{}, err
	}
	result := data.Pages.Search
	if limit > 0 && len(result.Results) > limit {
		result.Results = result.Results[:limit]
	}
	return result, nil
}

func (c *Client) GetPage(ctx context.Context, idOrPath string, locale string, withChildren bool) (Page, error) {
	if locale == "" {
		locale = c.defaultLocale
	}
	var page Page
	id, isID := parsePositiveID(idOrPath)
	if isID {
		const query = `query($id: Int!) { pages { single(id: $id) { id path title description content render locale createdAt updatedAt authorName tags { tag } isPublished isPrivate } } }`
		var data struct {
			Pages struct {
				Single *Page `json:"single"`
			} `json:"pages"`
		}
		if err := c.graphql(ctx, query, map[string]any{"id": id}, &data); err != nil {
			if isNotFoundGraphQLError(err) {
				return Page{}, fmt.Errorf("%w: page %s", ErrNotFound, idOrPath)
			}
			return Page{}, err
		}
		if data.Pages.Single == nil {
			return Page{}, fmt.Errorf("%w: page %s", ErrNotFound, idOrPath)
		}
		page = *data.Pages.Single
	} else {
		const query = `query($path: String!, $locale: String!) { pages { singleByPath(path: $path, locale: $locale) { id path title description content render locale createdAt updatedAt authorName tags { tag } isPublished isPrivate } } }`
		var data struct {
			Pages struct {
				SingleByPath *Page `json:"singleByPath"`
			} `json:"pages"`
		}
		if err := c.graphql(ctx, query, map[string]any{"path": trimLeadingSlash(idOrPath), "locale": locale}, &data); err != nil {
			if isNotFoundGraphQLError(err) {
				return Page{}, fmt.Errorf("%w: page %s", ErrNotFound, idOrPath)
			}
			return Page{}, err
		}
		if data.Pages.SingleByPath == nil {
			return Page{}, fmt.Errorf("%w: page %s", ErrNotFound, idOrPath)
		}
		page = *data.Pages.SingleByPath
	}
	if withChildren {
		pages, err := c.ListPages(ctx, ListOptions{Limit: 0})
		if err != nil {
			return Page{}, err
		}
		prefix := strings.Trim(page.Path, "/") + "/"
		for _, child := range pages {
			if child.ID != page.ID && strings.HasPrefix(strings.Trim(child.Path, "/"), prefix) {
				page.Children = append(page.Children, child)
			}
		}
	}
	return page, nil
}

func (c *Client) CreatePage(ctx context.Context, input CreatePageInput) (Page, error) {
	if input.Locale == "" {
		input.Locale = c.defaultLocale
	}
	if input.Editor == "" {
		input.Editor = c.defaultEditor
	}
	if err := validatePath(input.Path); err != nil {
		return Page{}, err
	}
	tags := input.Tags
	if tags == nil {
		tags = []string{}
	}
	const mutation = `mutation($content: String!, $description: String!, $editor: String!, $isPrivate: Boolean!, $isPublished: Boolean!, $locale: String!, $path: String!, $tags: [String]!, $title: String!) { pages { create(content: $content, description: $description, editor: $editor, isPrivate: $isPrivate, isPublished: $isPublished, locale: $locale, path: $path, tags: $tags, title: $title) { responseResult { succeeded errorCode message } page { id path title } } } }`
	var data struct {
		Pages struct {
			Create mutationResult `json:"create"`
		} `json:"pages"`
	}
	vars := map[string]any{
		"content": input.Content, "description": input.Description, "editor": input.Editor,
		"isPrivate": input.IsPrivate, "isPublished": input.IsPublished, "locale": input.Locale,
		"path": trimLeadingSlash(input.Path), "tags": tags, "title": input.Title,
	}
	if err := c.graphql(ctx, mutation, vars, &data); err != nil {
		return Page{}, err
	}
	return data.Pages.Create.pageOrError("create page")
}

func (c *Client) UpdatePage(ctx context.Context, input UpdatePageInput) (Page, error) {
	if input.ID < 1 {
		return Page{}, fmt.Errorf("%w: invalid id %d", ErrValidation, input.ID)
	}
	current, err := c.GetPage(ctx, strconv.Itoa(input.ID), "", false)
	if err != nil {
		return Page{}, err
	}
	content := current.Content
	if input.Content != nil {
		content = *input.Content
	}
	title := current.Title
	if input.Title != nil {
		title = *input.Title
	}
	description := current.Description
	if input.Description != nil {
		description = *input.Description
	}
	tags := []string(current.Tags)
	if input.SetTags {
		tags = input.Tags
		if tags == nil {
			tags = []string{}
		}
	}
	isPublished := current.IsPublished
	if input.IsPublished != nil {
		isPublished = *input.IsPublished
	}
	const mutation = `mutation($id: Int!, $content: String!, $description: String!, $isPublished: Boolean!, $tags: [String]!, $title: String!) { pages { update(id: $id, content: $content, description: $description, isPublished: $isPublished, tags: $tags, title: $title) { responseResult { succeeded errorCode message } page { id path title updatedAt } } } }`
	var data struct {
		Pages struct {
			Update mutationResult `json:"update"`
		} `json:"pages"`
	}
	vars := map[string]any{"id": input.ID, "content": content, "description": description, "isPublished": isPublished, "tags": tags, "title": title}
	if err := c.graphql(ctx, mutation, vars, &data); err != nil {
		return Page{}, err
	}
	return data.Pages.Update.pageOrError("update page")
}

func (c *Client) MovePage(ctx context.Context, id int, newPath string, locale string) error {
	if id < 1 {
		return fmt.Errorf("%w: invalid id %d", ErrValidation, id)
	}
	if locale == "" {
		locale = c.defaultLocale
	}
	if err := validatePath(newPath); err != nil {
		return err
	}
	const mutation = `mutation($id: Int!, $path: String!, $locale: String!) { pages { move(id: $id, destinationPath: $path, destinationLocale: $locale) { responseResult { succeeded errorCode message } } } }`
	var data struct {
		Pages struct {
			Move mutationResult `json:"move"`
		} `json:"pages"`
	}
	if err := c.graphql(ctx, mutation, map[string]any{"id": id, "path": trimLeadingSlash(newPath), "locale": locale}, &data); err != nil {
		return err
	}
	return data.Pages.Move.err("move page")
}

func (c *Client) DeletePage(ctx context.Context, id int) error {
	if id < 1 {
		return fmt.Errorf("%w: invalid id %d", ErrValidation, id)
	}
	const mutation = `mutation($id: Int!) { pages { delete(id: $id) { responseResult { succeeded errorCode message } } } }`
	var data struct {
		Pages struct {
			Delete mutationResult `json:"delete"`
		} `json:"pages"`
	}
	if err := c.graphql(ctx, mutation, map[string]any{"id": id}, &data); err != nil {
		return err
	}
	return data.Pages.Delete.err("delete page")
}

func (c *Client) ListTags(ctx context.Context) ([]Tag, error) {
	const query = `query { pages { tags { id tag title createdAt updatedAt } } }`
	var data struct {
		Pages struct {
			Tags []Tag `json:"tags"`
		} `json:"pages"`
	}
	if err := c.graphql(ctx, query, nil, &data); err != nil {
		return nil, err
	}
	return data.Pages.Tags, nil
}

func (c *Client) ListAssets(ctx context.Context, folder string, limit int) ([]Asset, error) {
	folder = normalizeAssetPath(folder)
	assets, err := c.listAssetsRecursive(ctx, 0, "", folder, map[int]bool{})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(assets, func(i, j int) bool { return assets[i].ID < assets[j].ID })
	if limit > 0 && len(assets) > limit {
		assets = assets[:limit]
	}
	return assets, nil
}

func (c *Client) listAssetsRecursive(ctx context.Context, folderID int, folderPath, filter string, visited map[int]bool) ([]Asset, error) {
	if visited[folderID] {
		return nil, nil
	}
	visited[folderID] = true

	const query = `query($folderID: Int!) { assets { list(folderId: $folderID, kind: ALL) { id filename ext kind mime fileSize createdAt updatedAt folder { id slug name } } folders(parentFolderId: $folderID) { id slug name } } }`
	var data struct {
		Assets struct {
			List    []Asset       `json:"list"`
			Folders []AssetFolder `json:"folders"`
		} `json:"assets"`
	}
	if err := c.graphql(ctx, query, map[string]any{"folderID": folderID}, &data); err != nil {
		return nil, err
	}

	assets := make([]Asset, 0, len(data.Assets.List))
	for _, asset := range data.Assets.List {
		asset.Filename = joinAssetPath(folderPath, asset.Filename)
		if filter != "" && !assetPathInFolder(asset.Filename, filter) {
			continue
		}
		assets = append(assets, asset)
	}

	for _, folder := range data.Assets.Folders {
		childPath := joinAssetPath(folderPath, folder.Slug)
		childAssets, err := c.listAssetsRecursive(ctx, folder.ID, childPath, filter, visited)
		if err != nil {
			return nil, err
		}
		assets = append(assets, childAssets...)
	}
	return assets, nil
}

func joinAssetPath(parent, child string) string {
	parent = normalizeAssetPath(parent)
	child = normalizeAssetPath(child)
	if parent == "" {
		return child
	}
	if child == "" {
		return parent
	}
	return pathpkg.Join(parent, child)
}

func normalizeAssetPath(value string) string {
	value = strings.TrimSpace(strings.Trim(value, "/"))
	if value == "" {
		return ""
	}
	value = pathpkg.Clean(value)
	if value == "." {
		return ""
	}
	return strings.Trim(value, "/")
}

func assetPathInFolder(filename, folder string) bool {
	filename = normalizeAssetPath(filename)
	folder = normalizeAssetPath(folder)
	return filename == folder || strings.HasPrefix(filename, folder+"/")
}

func (c *Client) UploadAsset(ctx context.Context, filePath, rename string) (map[string]any, error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	name := rename
	if name == "" {
		name = filepath.Base(filePath)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/u", pr)
	if err != nil {
		_ = file.Close()
		_ = pw.Close()
		return nil, err
	}
	go func() {
		defer file.Close()
		meta, err := writer.CreateFormField("mediaUpload")
		if err == nil {
			_, err = meta.Write([]byte(`{"folderId":0}`))
		}
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		part, err := writer.CreateFormFile("mediaUpload", name)
		if err == nil {
			_, err = io.Copy(part, file)
		}
		if closeErr := writer.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.logf("POST /u")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	c.logf("POST /u -> %s", resp.Status)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("http %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return map[string]any{"ok": true}, nil
	}
	return out, nil
}

func (c *Client) DeleteAsset(ctx context.Context, id int) error {
	if id < 1 {
		return fmt.Errorf("%w: invalid id %d", ErrValidation, id)
	}
	const mutation = `mutation($id: Int!) { assets { deleteAsset(id: $id) { responseResult { succeeded errorCode message } } } }`
	var data struct {
		Assets struct {
			DeleteAsset mutationResult `json:"deleteAsset"`
		} `json:"assets"`
	}
	if err := c.graphql(ctx, mutation, map[string]any{"id": id}, &data); err != nil {
		return err
	}
	return data.Assets.DeleteAsset.err("delete asset")
}

func (c *Client) Health(ctx context.Context) (SystemInfo, error) {
	const query = `query { system { info { configFile currentVersion latestVersion operatingSystem hostname platform } } }`
	var data struct {
		System struct {
			Info SystemInfo `json:"info"`
		} `json:"system"`
	}
	if err := c.graphql(ctx, query, nil, &data); err != nil {
		return SystemInfo{}, err
	}
	return data.System.Info, nil
}

func (c *Client) Stats(ctx context.Context) (Stats, error) {
	pages, err := c.ListPages(ctx, ListOptions{Limit: 0})
	if err != nil {
		return Stats{}, err
	}
	stats := Stats{TotalPages: len(pages), Locales: map[string]int{}, TopTags: map[string]int{}}
	for _, page := range pages {
		if page.IsPublished {
			stats.PublishedPages++
		} else {
			stats.DraftPages++
		}
		stats.Locales[page.Locale]++
		for _, tag := range page.Tags {
			stats.TopTags[tag]++
		}
	}
	return stats, nil
}

func (c *Client) PageVersions(ctx context.Context, id int) ([]Version, error) {
	if id < 1 {
		return nil, fmt.Errorf("%w: invalid id %d", ErrValidation, id)
	}
	const query = `query($id: Int!) { pages { history(id: $id) { trail { versionId versionDate authorName actionType } total } } }`
	var data struct {
		Pages struct {
			History struct {
				Trail []Version `json:"trail"`
			} `json:"history"`
		} `json:"pages"`
	}
	if err := c.graphql(ctx, query, map[string]any{"id": id}, &data); err != nil {
		return nil, err
	}
	return data.Pages.History.Trail, nil
}

func (c *Client) GetPageVersion(ctx context.Context, pageID, versionID int) (PageVersion, error) {
	if pageID < 1 || versionID < 1 {
		return PageVersion{}, fmt.Errorf("%w: invalid page or version id", ErrValidation)
	}
	const query = `query($pageID: Int!, $versionID: Int!) { pages { version(pageId: $pageID, versionId: $versionID) { versionId versionDate authorName actionType: action path title content } } }`
	var data struct {
		Pages struct {
			Version *PageVersion `json:"version"`
		} `json:"pages"`
	}
	if err := c.graphql(ctx, query, map[string]any{"pageID": pageID, "versionID": versionID}, &data); err != nil {
		return PageVersion{}, err
	}
	if data.Pages.Version == nil {
		return PageVersion{}, fmt.Errorf("%w: page %d version %d", ErrNotFound, pageID, versionID)
	}
	return *data.Pages.Version, nil
}

func (c *Client) RevertPage(ctx context.Context, pageID, versionID int) error {
	if pageID < 1 || versionID < 1 {
		return fmt.Errorf("%w: invalid page or version id", ErrValidation)
	}
	const mutation = `mutation($pageID: Int!, $versionID: Int!) { pages { restore(pageId: $pageID, versionId: $versionID) { responseResult { succeeded errorCode message } } } }`
	var data struct {
		Pages struct {
			Restore mutationResult `json:"restore"`
		} `json:"pages"`
	}
	if err := c.graphql(ctx, mutation, map[string]any{"pageID": pageID, "versionID": versionID}, &data); err != nil {
		return err
	}
	return data.Pages.Restore.err("revert page")
}

type mutationResult struct {
	ResponseResult struct {
		Succeeded bool         `json:"succeeded"`
		ErrorCode responseCode `json:"errorCode"`
		Message   string       `json:"message"`
	} `json:"responseResult"`
	Page Page `json:"page"`
}

func (r mutationResult) err(action string) error {
	if r.ResponseResult.Succeeded {
		return nil
	}
	msg := r.ResponseResult.Message
	if msg == "" {
		msg = "failed to " + action
	}
	if r.ResponseResult.ErrorCode != "" {
		msg += " (" + string(r.ResponseResult.ErrorCode) + ")"
	}
	return errors.New(msg)
}

func (r mutationResult) pageOrError(action string) (Page, error) {
	if err := r.err(action); err != nil {
		return Page{}, err
	}
	return r.Page, nil
}

func parsePositiveID(input string) (int, bool) {
	id, err := strconv.Atoi(input)
	return id, err == nil && id > 0 && strconv.Itoa(id) == input
}

type responseCode string

func (c *responseCode) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*c = ""
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*c = responseCode(text)
		return nil
	}
	var number int
	if err := json.Unmarshal(data, &number); err == nil {
		*c = responseCode(strconv.Itoa(number))
		return nil
	}
	return fmt.Errorf("unsupported response code shape: %s", string(data))
}

func isNotFoundGraphQLError(err error) bool {
	var gqlErrs GraphQLErrors
	if !errors.As(err, &gqlErrs) {
		return false
	}
	for _, gqlErr := range gqlErrs {
		if strings.Contains(strings.ToLower(gqlErr.Message), "does not exist") {
			return true
		}
	}
	return false
}

func validatePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("%w: path is required", ErrValidation)
	}
	if strings.ContainsAny(trimLeadingSlash(path), `<>:"|?*`) {
		return fmt.Errorf("%w: invalid characters in path %q", ErrValidation, path)
	}
	return nil
}

func trimLeadingSlash(path string) string {
	return strings.TrimPrefix(path, "/")
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func (c *Client) logf(format string, args ...any) {
	if c.logger != nil {
		c.logger(format, args...)
	}
}

func redactVariables(variables map[string]any) string {
	if variables == nil {
		return "{}"
	}
	clean := make(map[string]any, len(variables))
	for key, value := range variables {
		if strings.Contains(strings.ToLower(key), "token") {
			clean[key] = "<redacted>"
			continue
		}
		clean[key] = value
	}
	data, err := json.Marshal(clean)
	if err != nil {
		return "<unavailable>"
	}
	return string(data)
}
