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

func IsNotFound(err error) bool        { return errors.Is(err, ErrNotFound) }
func IsUniqueViolation(err error) bool { return errors.Is(err, ErrUniqueViolation) }
