package repository

import (
	"context"
	"time"
)

const (
	StorageDriverSQLite = "sqlite"
)

type Repository interface {
	Close() error
	Ping(context.Context) error

	GetAdminByEmail(context.Context, string) (Admin, error)
	UpsertAdmin(context.Context, string, string, bool) (Admin, error)
	CreateSession(context.Context, string, string, time.Time, string, string) (Session, error)
	GetSessionWithAdminByTokenHash(context.Context, string) (Session, Admin, error)
	TouchSession(context.Context, string) error
	DeleteSessionByHash(context.Context, string) error

	UpsertServiceState(context.Context, string, string, *string, string) error
	GetServiceState(context.Context, string) (ServiceState, error)

	InsertSystemSnapshot(context.Context, SystemSnapshot) (SystemSnapshot, error)
	ListSystemSnapshots(context.Context, time.Time, time.Time, int) ([]SystemSnapshot, error)
}

type OpenOptions struct {
	StorageRoot string
	SQLitePath  string
}

func Open(opts OpenOptions) (Repository, error) {
	if opts.SQLitePath != "" {
		return NewSQLiteRepository(opts.SQLitePath)
	}
	return NewSQLiteRepository(defaultSQLitePath(opts.StorageRoot))
}
