package domain

import "context"

type DashboardSummary struct {
	UID         string   `json:"uid"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	FolderUID   string   `json:"folderUid"`
	FolderTitle string   `json:"folderTitle"`
	Tags        []string `json:"tags"`
	IsStarred   bool     `json:"isStarred"`
}

type DashboardMeta struct {
	Type        string `json:"type"`
	URL         string `json:"url"`
	Version     int    `json:"version"`
	FolderUID   string `json:"folderUid"`
	FolderTitle string `json:"folderTitle"`
	FolderURL   string `json:"folderUrl"`
	IsStarred   bool   `json:"isStarred"`
	CanSave     bool   `json:"canSave"`
	CanEdit     bool   `json:"canEdit"`
	CanAdmin    bool   `json:"canAdmin"`
	Created     string `json:"created"`
	Updated     string `json:"updated"`
	CreatedBy   string `json:"createdBy"`
	UpdatedBy   string `json:"updatedBy"`
}

type Dashboard struct {
	Meta  DashboardMeta          `json:"meta"`
	Model map[string]interface{} `json:"dashboard"`
}

type DashboardSaveResult struct {
	ID      int    `json:"id"`
	UID     string `json:"uid"`
	URL     string `json:"url"`
	Status  string `json:"status"`
	Version int    `json:"version"`
}

type DashboardDeleteResult struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	UID     string `json:"uid"`
}

type Folder struct {
	ID    int    `json:"id"`
	UID   string `json:"uid"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type DashboardRepository interface {
	SearchDashboards(ctx context.Context, query, folderUID string, tags []string, limit int) ([]DashboardSummary, error)
	GetDashboard(ctx context.Context, uid string) (Dashboard, error)
	SaveDashboard(ctx context.Context, model map[string]interface{}, folderUID string, overwrite bool, message string) (DashboardSaveResult, error)
	DeleteDashboard(ctx context.Context, uid string) (DashboardDeleteResult, error)
	ListFolders(ctx context.Context) ([]Folder, error)
}
