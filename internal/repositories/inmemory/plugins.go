package inmemory

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/samber/lo"
)

type PluginRepository struct {
	mu      sync.RWMutex
	plugins map[uint]*domain.Plugin
	nextID  uint32
}

func NewPluginRepository() *PluginRepository {
	return &PluginRepository{
		plugins: make(map[uint]*domain.Plugin),
	}
}

func (r *PluginRepository) FindAll(
	_ context.Context,
	order []filters.Sorting,
	pagination *filters.Pagination,
) ([]domain.Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]domain.Plugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, r.copyPlugin(plugin))
	}

	r.sortPlugins(plugins, order)

	return r.applyPagination(plugins, pagination), nil
}

func (r *PluginRepository) Find(
	_ context.Context,
	filter *filters.FindPlugin,
	order []filters.Sorting,
	pagination *filters.Pagination,
) ([]domain.Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var plugins []domain.Plugin

	for _, plugin := range r.plugins {
		if r.matchesFilter(plugin, filter) {
			plugins = append(plugins, r.copyPlugin(plugin))
		}
	}

	r.sortPlugins(plugins, order)

	return r.applyPagination(plugins, pagination), nil
}

func (r *PluginRepository) Save(_ context.Context, plugin *domain.Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	plugin.UpdatedAt = lo.ToPtr(now)

	_, exists := r.plugins[plugin.ID]
	if !exists {
		if plugin.ID == 0 {
			plugin.ID = uint(atomic.AddUint32(&r.nextID, 1))
		}

		if plugin.CreatedAt == nil || plugin.CreatedAt.IsZero() {
			plugin.CreatedAt = lo.ToPtr(now)
		}
	}

	r.plugins[plugin.ID] = &domain.Plugin{
		ID:                  plugin.ID,
		Name:                plugin.Name,
		Version:             plugin.Version,
		Description:         plugin.Description,
		Author:              plugin.Author,
		APIVersion:          plugin.APIVersion,
		Filename:            plugin.Filename,
		Source:              plugin.Source,
		Homepage:            plugin.Homepage,
		RequiredPermissions: copyPermissions(plugin.RequiredPermissions),
		AllowedPermissions:  copyPermissions(plugin.AllowedPermissions),
		Status:              plugin.Status,
		Priority:            plugin.Priority,
		Category:            plugin.Category,
		Dependencies:        copyStrings(plugin.Dependencies),
		Config:              copyConfig(plugin.Config),
		InstalledAt:         plugin.InstalledAt,
		LastLoadedAt:        plugin.LastLoadedAt,
		CreatedAt:           plugin.CreatedAt,
		UpdatedAt:           plugin.UpdatedAt,
	}

	return nil
}

func (r *PluginRepository) Delete(_ context.Context, id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.plugins, id)

	return nil
}

func (r *PluginRepository) Exists(_ context.Context, filter *filters.FindPlugin) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, plugin := range r.plugins {
		if r.matchesFilter(plugin, filter) {
			return true, nil
		}
	}

	return false, nil
}

func (r *PluginRepository) matchesFilter(plugin *domain.Plugin, filter *filters.FindPlugin) bool {
	if filter == nil {
		return true
	}

	if len(filter.IDs) > 0 && !slices.Contains(filter.IDs, plugin.ID) {
		return false
	}

	if len(filter.Names) > 0 && !slices.Contains(filter.Names, plugin.Name) {
		return false
	}

	if len(filter.Statuses) > 0 && !slices.Contains(filter.Statuses, plugin.Status) {
		return false
	}

	if len(filter.Categories) > 0 {
		if plugin.Category == nil || !slices.Contains(filter.Categories, *plugin.Category) {
			return false
		}
	}

	return true
}

func (r *PluginRepository) sortPlugins(plugins []domain.Plugin, order []filters.Sorting) {
	if len(order) == 0 {
		sort.Slice(plugins, func(i, j int) bool {
			if plugins[i].Priority != plugins[j].Priority {
				return plugins[i].Priority > plugins[j].Priority
			}

			return plugins[i].Name < plugins[j].Name
		})

		return
	}

	sort.Slice(plugins, func(i, j int) bool {
		for _, sorting := range order {
			var result int
			switch sorting.Field {
			case "id":
				result = cmp.Compare(plugins[i].ID, plugins[j].ID)
			case "name":
				result = strings.Compare(plugins[i].Name, plugins[j].Name)
			case "priority":
				result = plugins[i].Priority - plugins[j].Priority
			case "status":
				result = strings.Compare(string(plugins[i].Status), string(plugins[j].Status))
			default:
				continue
			}

			if result != 0 {
				if sorting.Direction == filters.SortDirectionDesc {
					return result > 0
				}

				return result < 0
			}
		}

		return false
	})
}

func (r *PluginRepository) applyPagination(plugins []domain.Plugin, pagination *filters.Pagination) []domain.Plugin {
	if pagination == nil {
		return plugins
	}

	if pagination.Offset >= len(plugins) {
		return []domain.Plugin{}
	}

	start := pagination.Offset
	end := min(start+pagination.Limit, len(plugins))

	return plugins[start:end]
}

func (r *PluginRepository) copyPlugin(plugin *domain.Plugin) domain.Plugin {
	return domain.Plugin{
		ID:                  plugin.ID,
		Name:                plugin.Name,
		Version:             plugin.Version,
		Description:         plugin.Description,
		Author:              plugin.Author,
		APIVersion:          plugin.APIVersion,
		Filename:            plugin.Filename,
		Source:              plugin.Source,
		Homepage:            plugin.Homepage,
		RequiredPermissions: copyPermissions(plugin.RequiredPermissions),
		AllowedPermissions:  copyPermissions(plugin.AllowedPermissions),
		Status:              plugin.Status,
		Priority:            plugin.Priority,
		Category:            plugin.Category,
		Dependencies:        copyStrings(plugin.Dependencies),
		Config:              copyConfig(plugin.Config),
		InstalledAt:         plugin.InstalledAt,
		LastLoadedAt:        plugin.LastLoadedAt,
		CreatedAt:           plugin.CreatedAt,
		UpdatedAt:           plugin.UpdatedAt,
	}
}

func copyPermissions(permissions []domain.PluginPermission) []domain.PluginPermission {
	if permissions == nil {
		return nil
	}

	result := make([]domain.PluginPermission, len(permissions))
	copy(result, permissions)

	return result
}

func copyStrings(strs []string) []string {
	if strs == nil {
		return nil
	}

	result := make([]string, len(strs))
	copy(result, strs)

	return result
}

func copyConfig(config map[string]any) map[string]any {
	if config == nil {
		return nil
	}

	return maps.Clone(config)
}
