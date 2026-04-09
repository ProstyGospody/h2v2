package core

import (
	"context"
	"strings"
)

func (s *Store) ListUserAccessByIDs(ctx context.Context, ids []string) ([]UserAccess, error) {
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		if trimmed := normalizeString(id); trimmed != "" {
			placeholders = append(placeholders, "?")
			args = append(args, trimmed)
		}
	}
	if len(placeholders) == 0 {
		return nil, nil
	}
	query := `SELECT ` + userAccessSelectCols + ` FROM core_user_access WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := s.db.QueryContext(resolveCtx(ctx), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]UserAccess, 0, len(placeholders))
	for rows.Next() {
		item, err := scanUserAccess(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
