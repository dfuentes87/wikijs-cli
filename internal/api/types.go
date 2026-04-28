package api

import (
	"encoding/json"
	"fmt"
)

type Page struct {
	ID          int    `json:"id"`
	Path        string `json:"path"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content,omitempty"`
	Render      string `json:"render,omitempty"`
	Locale      string `json:"locale,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
	AuthorName  string `json:"authorName,omitempty"`
	Tags        Tags   `json:"tags,omitempty"`
	IsPublished bool   `json:"isPublished"`
	IsPrivate   bool   `json:"isPrivate,omitempty"`
	Children    []Page `json:"children,omitempty"`
}

type Tags []string

func (t *Tags) UnmarshalJSON(data []byte) error {
	var strings []string
	if err := json.Unmarshal(data, &strings); err == nil {
		*t = strings
		return nil
	}
	var objects []struct {
		Tag string `json:"tag"`
	}
	if err := json.Unmarshal(data, &objects); err == nil {
		out := make([]string, 0, len(objects))
		for _, item := range objects {
			if item.Tag != "" {
				out = append(out, item.Tag)
			}
		}
		*t = out
		return nil
	}
	return fmt.Errorf("unsupported tags shape: %s", string(data))
}

type SearchResult struct {
	Results     []Page   `json:"results"`
	TotalHits   int      `json:"totalHits"`
	Suggestions []string `json:"suggestions,omitempty"`
}

type Tag struct {
	ID        int    `json:"id"`
	Tag       string `json:"tag"`
	Title     string `json:"title,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type Asset struct {
	ID        int    `json:"id"`
	Filename  string `json:"filename"`
	Ext       string `json:"ext,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Mime      string `json:"mime,omitempty"`
	FileSize  int64  `json:"fileSize,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type SystemInfo struct {
	ConfigFile      string `json:"configFile,omitempty"`
	CurrentVersion  string `json:"currentVersion,omitempty"`
	LatestVersion   string `json:"latestVersion,omitempty"`
	OperatingSystem string `json:"operatingSystem,omitempty"`
	Hostname        string `json:"hostname,omitempty"`
	Platform        string `json:"platform,omitempty"`
}

type Stats struct {
	TotalPages     int            `json:"totalPages"`
	PublishedPages int            `json:"publishedPages"`
	DraftPages     int            `json:"draftPages"`
	Locales        map[string]int `json:"locales"`
	TopTags        map[string]int `json:"topTags"`
}

type Version struct {
	VersionID   int    `json:"versionId"`
	VersionDate string `json:"versionDate"`
	AuthorName  string `json:"authorName,omitempty"`
	ActionType  string `json:"actionType,omitempty"`
}

type PageVersion struct {
	VersionID   int    `json:"versionId"`
	VersionDate string `json:"versionDate,omitempty"`
	AuthorName  string `json:"authorName,omitempty"`
	ActionType  string `json:"actionType,omitempty"`
	Path        string `json:"path,omitempty"`
	Title       string `json:"title,omitempty"`
	Content     string `json:"content,omitempty"`
}

type CreatePageInput struct {
	Path        string
	Title       string
	Content     string
	Description string
	Tags        []string
	Locale      string
	Editor      string
	IsPublished bool
	IsPrivate   bool
}

type UpdatePageInput struct {
	ID          int
	Content     *string
	Title       *string
	Description *string
	Tags        []string
	SetTags     bool
	IsPublished *bool
}
