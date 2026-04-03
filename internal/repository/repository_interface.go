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

	InsertAuditLog(context.Context, *string, string, string, *string, any) error
	ListAuditLogs(context.Context, int, int) ([]AuditLog, error)
	UpsertServiceState(context.Context, string, string, *string, string) error
	GetServiceState(context.Context, string) (ServiceState, error)

	InsertSystemSnapshot(context.Context, SystemSnapshot) (SystemSnapshot, error)
	ListSystemSnapshots(context.Context, time.Time, time.Time, int) ([]SystemSnapshot, error)

	CreateUser(context.Context, CreateUserInput) (UserWithCredentials, error)
	ListUsers(context.Context, int, int, *Protocol) ([]UserWithCredentials, error)
	GetUser(context.Context, string) (UserWithCredentials, error)
	GetUserBySubject(context.Context, string) (UserWithCredentials, error)
	UpdateUser(context.Context, string, UpdateUserInput) (UserWithCredentials, error)
	DeleteUsers(context.Context, BatchDeleteUsersInput) error
	SetUsersStateBatch(context.Context, BatchUserStateInput) (int, error)
	KickUsers(context.Context, []string) ([]string, error)

	ListInbounds(context.Context, *Protocol) ([]Inbound, error)
	GetInbound(context.Context, string) (Inbound, error)
	UpsertInbound(context.Context, Inbound) (Inbound, error)
	DeleteInbound(context.Context, string) error

	GetSubscriptionToken(context.Context, string) (SubscriptionToken, error)
	EnsureSubscriptionToken(context.Context, string) (SubscriptionToken, error)
	RotateSubscriptionToken(context.Context, string) (SubscriptionToken, error)
	RevokeSubscriptionToken(context.Context, string) (SubscriptionToken, error)
	ClearSubscriptionRevocation(context.Context, string) (SubscriptionToken, error)

	InsertTrafficCounters(context.Context, []TrafficCounter) error
	ListTrafficCounters(context.Context, string, *Protocol, int, int) ([]TrafficCounter, error)
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
