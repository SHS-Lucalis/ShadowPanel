package getpluginversions

import (
	"time"

	"github.com/gameap/gameap/internal/services/pluginstore"
)

type screenshotResponse struct {
	ID        int    `json:"id"`
	URL       string `json:"url"`
	SortOrder int    `json:"sort_order"`
}

type versionResponse struct {
	ID                  int                  `json:"id"`
	Version             string               `json:"version"`
	Changelog           string               `json:"changelog"`
	FileSize            int64                `json:"file_size"`
	FileHash            string               `json:"file_hash"`
	SignURL             string               `json:"sign_url"`
	MinGameAPVersion    string               `json:"min_gameap_version"`
	MinPluginAPIVersion string               `json:"min_plugin_api_version"`
	IsStable            bool                 `json:"is_stable"`
	Screenshots         []screenshotResponse `json:"screenshots"`
	DownloadCount       int                  `json:"download_count"`
	CreatedAt           time.Time            `json:"created_at"`
}

type paginatedVersionsResponse struct {
	CurrentPage int               `json:"current_page"`
	Data        []versionResponse `json:"data"`
	From        int               `json:"from"`
	LastPage    int               `json:"last_page"`
	PerPage     int               `json:"per_page"`
	Total       int               `json:"total"`
}

func newVersionsResponse(
	versions *pluginstore.PaginatedResponse[pluginstore.PluginVersion],
) *paginatedVersionsResponse {
	data := make([]versionResponse, 0, len(versions.Data))

	for _, v := range versions.Data {
		screenshots := make([]screenshotResponse, 0, len(v.Screenshots))
		for _, s := range v.Screenshots {
			screenshots = append(screenshots, screenshotResponse{
				ID:        s.ID,
				URL:       s.URL,
				SortOrder: s.SortOrder,
			})
		}

		data = append(data, versionResponse{
			ID:                  v.ID,
			Version:             v.Version,
			Changelog:           v.Changelog,
			FileSize:            v.FileSize,
			FileHash:            v.FileHash,
			SignURL:             v.SignURL,
			MinGameAPVersion:    v.MinGameAPVersion,
			MinPluginAPIVersion: v.MinPluginAPIVersion,
			IsStable:            v.IsStable,
			Screenshots:         screenshots,
			DownloadCount:       v.DownloadCount,
			CreatedAt:           v.CreatedAt,
		})
	}

	return &paginatedVersionsResponse{
		CurrentPage: versions.CurrentPage,
		Data:        data,
		From:        versions.From,
		LastPage:    versions.LastPage,
		PerPage:     versions.PerPage,
		Total:       versions.Total,
	}
}
