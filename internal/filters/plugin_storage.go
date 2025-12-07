package filters

import "github.com/gameap/gameap/internal/domain"

type FindPluginStorage struct {
	IDs         []uint64
	PluginIDs   []uint64
	Keys        []string
	EntityPairs []domain.PluginStorageEntityPair
}
