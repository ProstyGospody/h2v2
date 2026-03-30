package repository

import (
	"context"
	"strings"
	"time"
)

const (
	StorageDriverFile   = "file"
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

	CreateHysteriaUser(context.Context, string, string, *string, *ClientOverrides) (HysteriaUser, error)
	ListHysteriaUsers(context.Context, int, int) ([]HysteriaUserView, error)
	ListEnabledHysteriaUsers(context.Context) ([]HysteriaUser, error)
	GetHysteriaUser(context.Context, string) (HysteriaUserView, error)
	UpdateHysteriaUser(context.Context, string, string, string, *string, *ClientOverrides) (HysteriaUserView, error)
	DeleteHysteriaUser(context.Context, string) error
	SetHysteriaUserEnabled(context.Context, string, bool) error
	TouchHysteriaUserLastSeen(context.Context, string, time.Time) error
	InsertHysteriaSnapshots(context.Context, []HysteriaSnapshot) error
	GetHysteriaStatsOverview(context.Context) (HysteriaOverview, error)
	ListHysteriaSnapshots(context.Context, string, int, int) ([]HysteriaSnapshot, error)

	InsertAuditLog(context.Context, *string, string, string, *string, any) error
	ListAuditLogs(context.Context, int, int) ([]AuditLog, error)
	UpsertServiceState(context.Context, string, string, *string, string) error
	GetServiceState(context.Context, string) (ServiceState, error)

	InsertSystemSnapshot(context.Context, SystemSnapshot) (SystemSnapshot, error)
	ListSystemSnapshots(context.Context, time.Time, time.Time, int) ([]SystemSnapshot, error)
}

type OpenOptions struct {
	Driver      string
	StorageRoot string
	AuditDir    string
	RuntimeDir  string
	SQLitePath  string
}

func Open(opts OpenOptions) (Repository, error) {
	driver := NormalizeDriver(opts.Driver)
	switch driver {
	case StorageDriverSQLite:
		return NewSQLiteRepository(opts.SQLitePath)
	default:
		return NewFileRepository(opts.StorageRoot, opts.AuditDir, opts.RuntimeDir)
	}
}

func NormalizeDriver(value string) string {
	driver := strings.ToLower(strings.TrimSpace(value))
	switch driver {
	case StorageDriverSQLite:
		return StorageDriverSQLite
	default:
		return StorageDriverFile
	}
}
