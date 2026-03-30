package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (r *SQLiteRepository) GetAdminByEmail(ctx context.Context, email string) (Admin, error) {
	row := r.db.QueryRowContext(
		resolveCtx(ctx),
		`SELECT id, email, password_hash, is_active, created_at_ns, updated_at_ns
		 FROM admins
		 WHERE email = ?
		 LIMIT 1`,
		strings.ToLower(strings.TrimSpace(email)),
	)
	return r.scanAdmin(row)
}

func (r *SQLiteRepository) UpsertAdmin(ctx context.Context, email string, passwordHash string, isActive bool) (Admin, error) {
	ctx = resolveCtx(ctx)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Admin{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	normalized := strings.ToLower(strings.TrimSpace(email))
	now := nowNano()

	var (
		id        string
		createdAt int64
	)
	scanErr := tx.QueryRowContext(
		ctx,
		`SELECT id, created_at_ns FROM admins WHERE email = ? LIMIT 1`,
		normalized,
	).Scan(&id, &createdAt)
	switch {
	case scanErr == nil:
		_, err = tx.ExecContext(
			ctx,
			`UPDATE admins
			 SET email = ?, password_hash = ?, is_active = ?, updated_at_ns = ?
			 WHERE id = ?`,
			normalized,
			passwordHash,
			sqliteBool(isActive),
			now,
			id,
		)
		if err != nil {
			return Admin{}, translateSQLiteErr(err)
		}
	case errors.Is(scanErr, sql.ErrNoRows):
		id = uuid.NewString()
		createdAt = now
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO admins (id, email, password_hash, is_active, created_at_ns, updated_at_ns)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			id,
			normalized,
			passwordHash,
			sqliteBool(isActive),
			createdAt,
			now,
		)
		if err != nil {
			return Admin{}, translateSQLiteErr(err)
		}
	default:
		return Admin{}, translateSQLiteErr(scanErr)
	}

	if err = tx.Commit(); err != nil {
		return Admin{}, err
	}

	return Admin{
		ID:           id,
		Email:        normalized,
		PasswordHash: passwordHash,
		IsActive:     isActive,
		CreatedAt:    fromUnixNano(createdAt),
		UpdatedAt:    fromUnixNano(now),
	}, nil
}

func (r *SQLiteRepository) CreateSession(ctx context.Context, adminID string, tokenHash string, expiresAt time.Time, ip string, userAgent string) (Session, error) {
	ctx = resolveCtx(ctx)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	adminID = strings.TrimSpace(adminID)
	if adminID == "" {
		return Session{}, ErrNotFound
	}
	var exists string
	if err = tx.QueryRowContext(ctx, `SELECT id FROM admins WHERE id = ? LIMIT 1`, adminID).Scan(&exists); err != nil {
		return Session{}, translateSQLiteErr(err)
	}

	now := nowNano()
	if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at_ns <= ?`, now); err != nil {
		return Session{}, err
	}

	session := Session{
		ID:               uuid.NewString(),
		AdminID:          adminID,
		SessionTokenHash: tokenHash,
		ExpiresAt:        expiresAt.UTC(),
		CreatedAt:        fromUnixNano(now),
		LastSeenAt:       fromUnixNano(now),
		IP:               strings.TrimSpace(ip),
		UserAgent:        strings.TrimSpace(userAgent),
	}
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO sessions (
			id, admin_id, session_token_hash, expires_at_ns, created_at_ns, last_seen_at_ns, ip, user_agent
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID,
		session.AdminID,
		session.SessionTokenHash,
		toUnixNano(session.ExpiresAt),
		toUnixNano(session.CreatedAt),
		toUnixNano(session.LastSeenAt),
		session.IP,
		session.UserAgent,
	)
	if err != nil {
		return Session{}, translateSQLiteErr(err)
	}

	if err = tx.Commit(); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (r *SQLiteRepository) GetSessionWithAdminByTokenHash(ctx context.Context, tokenHash string) (Session, Admin, error) {
	ctx = resolveCtx(ctx)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, Admin{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	now := nowNano()
	if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at_ns <= ?`, now); err != nil {
		return Session{}, Admin{}, err
	}

	row := tx.QueryRowContext(
		ctx,
		`SELECT
			s.id, s.admin_id, s.session_token_hash, s.expires_at_ns, s.created_at_ns, s.last_seen_at_ns, s.ip, s.user_agent,
			a.id, a.email, a.password_hash, a.is_active, a.created_at_ns, a.updated_at_ns
		 FROM sessions s
		 JOIN admins a ON a.id = s.admin_id
		 WHERE s.session_token_hash = ?
		 LIMIT 1`,
		tokenHash,
	)
	var (
		session                     Session
		sessionExpiresAt            int64
		sessionCreatedAt            int64
		sessionLastSeenAt           int64
		admin                       Admin
		adminIsActive               int64
		adminCreatedAt, adminUpdatedAt int64
	)
	if err = row.Scan(
		&session.ID,
		&session.AdminID,
		&session.SessionTokenHash,
		&sessionExpiresAt,
		&sessionCreatedAt,
		&sessionLastSeenAt,
		&session.IP,
		&session.UserAgent,
		&admin.ID,
		&admin.Email,
		&admin.PasswordHash,
		&adminIsActive,
		&adminCreatedAt,
		&adminUpdatedAt,
	); err != nil {
		return Session{}, Admin{}, translateSQLiteErr(err)
	}
	session.ExpiresAt = fromUnixNano(sessionExpiresAt)
	session.CreatedAt = fromUnixNano(sessionCreatedAt)
	session.LastSeenAt = fromUnixNano(sessionLastSeenAt)
	admin.IsActive = boolFromSQLite(adminIsActive)
	admin.CreatedAt = fromUnixNano(adminCreatedAt)
	admin.UpdatedAt = fromUnixNano(adminUpdatedAt)

	if !session.ExpiresAt.After(fromUnixNano(now)) {
		return Session{}, Admin{}, ErrNotFound
	}

	if err = tx.Commit(); err != nil {
		return Session{}, Admin{}, err
	}
	return session, admin, nil
}

func (r *SQLiteRepository) TouchSession(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(
		resolveCtx(ctx),
		`UPDATE sessions SET last_seen_at_ns = ? WHERE id = ?`,
		nowNano(),
		strings.TrimSpace(sessionID),
	)
	return err
}

func (r *SQLiteRepository) DeleteSessionByHash(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(resolveCtx(ctx), `DELETE FROM sessions WHERE session_token_hash = ?`, tokenHash)
	return err
}
