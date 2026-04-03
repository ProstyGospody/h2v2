package repository

import (
	"errors"
	"path/filepath"
	"strings"
)

var (
	ErrNotFound        = errors.New("repository: not found")
	ErrUniqueViolation = errors.New("repository: unique violation")
)

func New(storageRoot string, _ string, _ string) (Repository, error) {
	return NewSQLiteRepository(defaultSQLitePath(storageRoot))
}

func defaultSQLitePath(storageRoot string) string {
	root := strings.TrimSpace(storageRoot)
	if root == "" {
		root = "/var/lib/h2v2"
	}
	return filepath.Join(root, "data", "h2v2.db")
}

func paginate[T any](items []T, limit int, offset int) []T {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = len(items)
	}
	if offset >= len(items) {
		return []T{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return append([]T(nil), items[offset:end]...)
}

func cleanOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullValue(value *string) any {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func maxInt(a int, b int) int {
	if a >= b {
		return a
	}
	return b
}

func IsNotFound(err error) bool        { return errors.Is(err, ErrNotFound) }
func IsUniqueViolation(err error) bool { return errors.Is(err, ErrUniqueViolation) }
