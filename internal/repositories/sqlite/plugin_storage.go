package sqlite

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories/base"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

type PluginStorageRepository struct {
	db base.DB
}

func NewPluginStorageRepository(db base.DB) *PluginStorageRepository {
	return &PluginStorageRepository{
		db: db,
	}
}

func (r *PluginStorageRepository) Find(
	ctx context.Context,
	filter *filters.FindPluginStorage,
	order []filters.Sorting,
	pagination *filters.Pagination,
) ([]domain.PluginStorageEntry, error) {
	builder := sq.Select(base.PluginStorageFields...).
		From(base.PluginStorageTable).
		Where(r.filterToSq(filter))

	return r.find(ctx, builder, order, pagination)
}

func (r *PluginStorageRepository) find(
	ctx context.Context,
	builder sq.SelectBuilder,
	order []filters.Sorting,
	pagination *filters.Pagination,
) ([]domain.PluginStorageEntry, error) {
	if len(order) > 0 {
		for _, o := range order {
			builder = builder.OrderBy(o.String())
		}
	} else {
		builder = builder.OrderBy("id ASC")
	}

	if pagination != nil {
		if pagination.Limit <= 0 {
			pagination.Limit = filters.DefaultLimit
		}

		if pagination.Offset < 0 {
			pagination.Offset = 0
		}

		builder = builder.Limit(uint64(pagination.Limit)).Offset(uint64(pagination.Offset))
	}

	query, args, err := builder.ToSql()
	if err != nil {
		return nil, errors.WithMessage(err, "failed to build query")
	}

	rows, err := r.db.QueryContext(ctx, query, args...) //nolint:sqlclosecheck
	if err != nil {
		return nil, errors.WithMessage(err, "failed to execute query")
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			slog.ErrorContext(ctx, "failed to close rows stream", "query", query, "err", err)
		}
	}(rows)

	var entries []domain.PluginStorageEntry

	for rows.Next() {
		var entry *domain.PluginStorageEntry
		entry, err = r.scan(rows)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to scan row")
		}

		entries = append(entries, *entry)
	}

	if err = rows.Err(); err != nil {
		return nil, errors.WithMessage(err, "rows iteration error")
	}

	return entries, nil
}

func (r *PluginStorageRepository) Save(ctx context.Context, entry *domain.PluginStorageEntry) error {
	now := time.Now()
	entry.UpdatedAt = &now

	existingID, err := r.findExistingEntryID(ctx, entry)
	if err != nil {
		return errors.WithMessage(err, "failed to find existing entry")
	}

	if existingID > 0 {
		return r.update(ctx, existingID, entry)
	}

	return r.insert(ctx, entry, &now)
}

func (r *PluginStorageRepository) findExistingEntryID(
	ctx context.Context,
	entry *domain.PluginStorageEntry,
) (uint64, error) {
	if entry.ID > 0 {
		return entry.ID, nil
	}

	builder := sq.Select("id").
		From(base.PluginStorageTable).
		Where(sq.Eq{
			"plugin_id":   entry.PluginID,
			"key":         entry.Key,
			"entity_type": entry.EntityType,
			"entity_id":   entry.EntityID,
		}).
		Limit(1)

	query, args, err := builder.ToSql()
	if err != nil {
		return 0, errors.WithMessage(err, "failed to build query")
	}

	var id uint64
	err = r.db.QueryRowContext(ctx, query, args...).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}

		return 0, errors.WithMessage(err, "failed to execute query")
	}

	return id, nil
}

func (r *PluginStorageRepository) insert(
	ctx context.Context,
	entry *domain.PluginStorageEntry,
	now *time.Time,
) error {
	entry.CreatedAt = now

	var createdAtStr, updatedAtStr *string
	if entry.CreatedAt != nil {
		createdAtStr = lo.ToPtr(entry.CreatedAt.Format(time.RFC3339Nano))
	}
	if entry.UpdatedAt != nil {
		updatedAtStr = lo.ToPtr(entry.UpdatedAt.Format(time.RFC3339Nano))
	}

	query, args, err := sq.Insert(base.PluginStorageTable).
		Columns(base.PluginStorageFields...).
		Values(
			nil,
			entry.PluginID,
			entry.Key,
			entry.EntityType,
			entry.EntityID,
			entry.Payload,
			createdAtStr,
			updatedAtStr,
		).
		Suffix("RETURNING id").
		ToSql()
	if err != nil {
		return errors.WithMessage(err, "failed to build query")
	}

	var returnedID uint64
	err = r.db.QueryRowContext(ctx, query, args...).Scan(&returnedID)
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}

	entry.ID = returnedID

	return nil
}

func (r *PluginStorageRepository) update(ctx context.Context, id uint64, entry *domain.PluginStorageEntry) error {
	entry.ID = id

	if entry.CreatedAt == nil || entry.CreatedAt.IsZero() {
		entry.CreatedAt = lo.ToPtr(time.Now())
	}

	var updatedAtStr *string
	if entry.UpdatedAt != nil {
		updatedAtStr = lo.ToPtr(entry.UpdatedAt.Format(time.RFC3339Nano))
	}

	query, args, err := sq.Update(base.PluginStorageTable).
		Set("payload", entry.Payload).
		Set("updated_at", updatedAtStr).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return errors.WithMessage(err, "failed to build query")
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}

	return nil
}

func (r *PluginStorageRepository) Delete(ctx context.Context, id uint64) error {
	query, args, err := sq.Delete(base.PluginStorageTable).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return errors.WithMessage(err, "failed to build query")
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}

	return nil
}

func (r *PluginStorageRepository) DeleteByPlugin(ctx context.Context, pluginID uint64) error {
	query, args, err := sq.Delete(base.PluginStorageTable).
		Where(sq.Eq{"plugin_id": pluginID}).
		ToSql()
	if err != nil {
		return errors.WithMessage(err, "failed to build query")
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}

	return nil
}

func (r *PluginStorageRepository) scan(row base.Scanner) (*domain.PluginStorageEntry, error) {
	var entry domain.PluginStorageEntry
	var createdAtStr, updatedAtStr *string

	err := row.Scan(
		&entry.ID,
		&entry.PluginID,
		&entry.Key,
		&entry.EntityType,
		&entry.EntityID,
		&entry.Payload,
		&createdAtStr,
		&updatedAtStr,
	)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to scan row")
	}

	if createdAtStr != nil && *createdAtStr != "" {
		createdAt, err := base.ParseTime(*createdAtStr)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to parse created_at time")
		}
		entry.CreatedAt = &createdAt
	}

	if updatedAtStr != nil && *updatedAtStr != "" {
		updatedAt, err := base.ParseTime(*updatedAtStr)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to parse updated_at time")
		}
		entry.UpdatedAt = &updatedAt
	}

	return &entry, nil
}

func (r *PluginStorageRepository) filterToSq(filter *filters.FindPluginStorage) sq.Sqlizer {
	if filter == nil {
		return nil
	}

	and := make(sq.And, 0, 4)

	if len(filter.IDs) > 0 {
		and = append(and, sq.Eq{"id": filter.IDs})
	}

	if len(filter.PluginIDs) > 0 {
		and = append(and, sq.Eq{"plugin_id": filter.PluginIDs})
	}

	if len(filter.Keys) > 0 {
		and = append(and, sq.Eq{"key": filter.Keys})
	}

	if len(filter.EntityPairs) > 0 {
		or := make(sq.Or, 0, len(filter.EntityPairs))
		for _, pair := range filter.EntityPairs {
			pairAnd := sq.And{
				sq.Eq{"entity_type": pair.EntityType},
				sq.Eq{"entity_id": pair.EntityID},
			}
			or = append(or, pairAnd)
		}
		and = append(and, or)
	}

	return and
}
