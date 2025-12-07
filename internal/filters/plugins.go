package filters

import "github.com/gameap/gameap/internal/domain"

type FindPlugin struct {
	IDs        []uint
	Names      []string
	Statuses   []domain.PluginStatus
	Categories []string
}

func (f *FindPlugin) FilterCount() int {
	count := 0
	if len(f.IDs) > 0 {
		count++
	}
	if len(f.Names) > 0 {
		count++
	}
	if len(f.Statuses) > 0 {
		count++
	}
	if len(f.Categories) > 0 {
		count++
	}

	return count
}

func FindPluginByIDs(ids ...uint) *FindPlugin {
	return &FindPlugin{
		IDs: ids,
	}
}

func FindPluginByNames(names ...string) *FindPlugin {
	return &FindPlugin{
		Names: names,
	}
}

func FindPluginByStatuses(statuses ...domain.PluginStatus) *FindPlugin {
	return &FindPlugin{
		Statuses: statuses,
	}
}
