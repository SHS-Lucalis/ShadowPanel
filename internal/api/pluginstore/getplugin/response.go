package getplugin

import (
	"time"

	"github.com/gameap/gameap/internal/services/pluginstore"
)

type authorResponse struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

type categoryResponse struct {
	ID          int    `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type labelResponse struct {
	ID    int    `json:"id"`
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type pluginDetailsResponse struct {
	ID                  string           `json:"id"`
	URL                 string           `json:"url"`
	Name                string           `json:"name"`
	Summary             string           `json:"summary"`
	Description         string           `json:"description"`
	IconURL             string           `json:"icon_url"`
	License             string           `json:"license"`
	RepositoryURL       string           `json:"repository_url"`
	MinGameAPVersion    string           `json:"min_gameap_version"`
	MinPluginAPIVersion string           `json:"min_plugin_api_version"`
	Author              authorResponse   `json:"author"`
	Category            categoryResponse `json:"category"`
	Labels              []labelResponse  `json:"labels"`
	DownloadCount       int              `json:"download_count"`
	RatingAvg           float64          `json:"rating_avg"`
	RatingCount         int              `json:"rating_count"`
	LatestVersion       string           `json:"latest_version"`
	PublishedAt         time.Time        `json:"published_at"`
	CreatedAt           time.Time        `json:"created_at"`
	UpdatedAt           time.Time        `json:"updated_at"`
	Installed           bool             `json:"installed"`
	InstalledVersion    *string          `json:"installed_version,omitempty"`
}

func newPluginResponse(plugin *pluginstore.PluginDetails, installedVersion *string) *pluginDetailsResponse {
	labels := make([]labelResponse, 0, len(plugin.Labels))
	for _, l := range plugin.Labels {
		labels = append(labels, labelResponse{
			ID:    l.ID,
			Slug:  l.Slug,
			Name:  l.Name,
			Color: l.Color,
		})
	}

	return &pluginDetailsResponse{
		ID:                  plugin.ID,
		URL:                 plugin.URL,
		Name:                plugin.Name,
		Summary:             plugin.Summary,
		Description:         plugin.Description,
		IconURL:             plugin.IconURL,
		License:             plugin.License,
		RepositoryURL:       plugin.RepositoryURL,
		MinGameAPVersion:    plugin.MinGameAPVersion,
		MinPluginAPIVersion: plugin.MinPluginAPIVersion,
		Author: authorResponse{
			ID:       plugin.Author.ID,
			Username: plugin.Author.Username,
		},
		Category: categoryResponse{
			ID:          plugin.Category.ID,
			Slug:        plugin.Category.Slug,
			Name:        plugin.Category.Name,
			Description: plugin.Category.Description,
			Icon:        plugin.Category.Icon,
		},
		Labels:           labels,
		DownloadCount:    plugin.DownloadCount,
		RatingAvg:        plugin.RatingAvg,
		RatingCount:      plugin.RatingCount,
		LatestVersion:    plugin.LatestVersion,
		PublishedAt:      plugin.PublishedAt,
		CreatedAt:        plugin.CreatedAt,
		UpdatedAt:        plugin.UpdatedAt,
		Installed:        installedVersion != nil,
		InstalledVersion: installedVersion,
	}
}
