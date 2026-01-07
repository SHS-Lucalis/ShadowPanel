package getplugins

import (
	"time"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/services/pluginstore"
)

type labelResponse struct {
	ID    int    `json:"id"`
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type categoryResponse struct {
	ID   int    `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type pluginResponse struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Summary          string           `json:"summary"`
	IconURL          string           `json:"icon_url"`
	Category         categoryResponse `json:"category"`
	Labels           []labelResponse  `json:"labels"`
	DownloadCount    int              `json:"download_count"`
	RatingAvg        float64          `json:"rating_avg"`
	RatingCount      int              `json:"rating_count"`
	LatestVersion    string           `json:"latest_version"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	Installed        bool             `json:"installed"`
	InstalledVersion *string          `json:"installed_version,omitempty"`
}

type paginatedPluginsResponse struct {
	CurrentPage int              `json:"current_page"`
	Data        []pluginResponse `json:"data"`
	From        int              `json:"from"`
	LastPage    int              `json:"last_page"`
	PerPage     int              `json:"per_page"`
	Total       int              `json:"total"`
}

func newPluginsResponse(
	storePlugins *pluginstore.PaginatedResponse[pluginstore.Plugin],
	installedMap map[domain.Uint64ID]string,
) *paginatedPluginsResponse {
	data := make([]pluginResponse, 0, len(storePlugins.Data))

	for _, p := range storePlugins.Data {
		labels := make([]labelResponse, 0, len(p.Labels))
		for _, l := range p.Labels {
			labels = append(labels, labelResponse{
				ID:    l.ID,
				Slug:  l.Slug,
				Name:  l.Name,
				Color: l.Color,
			})
		}

		data = append(data, pluginResponse{
			ID:      p.ID,
			Name:    p.Name,
			Summary: p.Summary,
			IconURL: p.IconURL,
			Category: categoryResponse{
				ID:   p.Category.ID,
				Slug: p.Category.Slug,
				Name: p.Category.Name,
			},
			Labels:           labels,
			DownloadCount:    p.DownloadCount,
			RatingAvg:        p.RatingAvg,
			RatingCount:      p.RatingCount,
			LatestVersion:    p.LatestVersion,
			CreatedAt:        p.CreatedAt,
			UpdatedAt:        p.UpdatedAt,
			Installed:        isInstalled(p.ID, installedMap),
			InstalledVersion: getInstalledVersion(p.ID, installedMap),
		})
	}

	return &paginatedPluginsResponse{
		CurrentPage: storePlugins.CurrentPage,
		Data:        data,
		From:        storePlugins.From,
		LastPage:    storePlugins.LastPage,
		PerPage:     storePlugins.PerPage,
		Total:       storePlugins.Total,
	}
}
