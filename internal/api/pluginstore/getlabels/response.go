package getlabels

import "github.com/gameap/gameap/internal/services/pluginstore"

type labelResponse struct {
	ID    int    `json:"id"`
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

func newLabelsResponse(labels []pluginstore.Label) []labelResponse {
	response := make([]labelResponse, 0, len(labels))

	for _, l := range labels {
		response = append(response, labelResponse{
			ID:    l.ID,
			Slug:  l.Slug,
			Name:  l.Name,
			Color: l.Color,
		})
	}

	return response
}
