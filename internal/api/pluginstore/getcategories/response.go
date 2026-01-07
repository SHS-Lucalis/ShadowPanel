package getcategories

import "github.com/gameap/gameap/internal/services/pluginstore"

type categoryResponse struct {
	ID          int    `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
}

func newCategoriesResponse(categories []pluginstore.Category) []categoryResponse {
	response := make([]categoryResponse, 0, len(categories))

	for _, c := range categories {
		response = append(response, categoryResponse{
			ID:          c.ID,
			Slug:        c.Slug,
			Name:        c.Name,
			Description: c.Description,
			SortOrder:   c.SortOrder,
		})
	}

	return response
}
