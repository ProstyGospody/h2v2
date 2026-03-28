package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (r *Repository) GetAdminByEmail(ctx context.Context, email string) (Admin, error) {
	var out Admin
	err := r.withLock(ctx, func() error {
		admins, err := r.loadAdminsNoLock()
		if err != nil {
			return err
		}
		needle := strings.ToLower(strings.TrimSpace(email))
		for _, admin := range admins {
			if strings.EqualFold(strings.TrimSpace(admin.Email), needle) {
				out = admin
				return nil
			}
		}
		return ErrNotFound
	})
	return out, err
}

func (r *Repository) UpsertAdmin(ctx context.Context, email string, passwordHash string, isActive bool) (Admin, error) {
	var out Admin
	err := r.withLock(ctx, func() error {
		normalizedEmail := strings.ToLower(strings.TrimSpace(email))
		now := time.Now().UTC()

		admins, err := r.loadAdminsNoLock()
		if err != nil {
			return err
		}
		for _, admin := range admins {
			if strings.EqualFold(strings.TrimSpace(admin.Email), normalizedEmail) {
				admin.Email = normalizedEmail
				admin.PasswordHash = passwordHash
				admin.IsActive = isActive
				admin.UpdatedAt = now
				if err := r.writeAdminNoLock(admin); err != nil {
					return err
				}
				out = admin
				return nil
			}
		}

		admin := Admin{ID: uuid.NewString(), Email: normalizedEmail, PasswordHash: passwordHash, IsActive: isActive, CreatedAt: now, UpdatedAt: now}
		if err := r.writeAdminNoLock(admin); err != nil {
			return err
		}
		out = admin
		return nil
	})
	return out, err
}

func (r *Repository) CreateSession(ctx context.Context, adminID string, tokenHash string, expiresAt time.Time, ip string, userAgent string) (Session, error) {
	var out Session
	err := r.withLock(ctx, func() error {
		if _, err := r.loadAdminNoLock(adminID); err != nil {
			return err
		}
		if err := r.purgeExpiredSessionsNoLock(time.Now().UTC()); err != nil {
			return err
		}
		sessions, err := r.loadSessionsNoLock()
		if err != nil {
			return err
		}
		for _, session := range sessions {
			if session.SessionTokenHash == tokenHash {
				return ErrUniqueViolation
			}
		}
		now := time.Now().UTC()
		session := Session{
			ID:               uuid.NewString(),
			AdminID:          adminID,
			SessionTokenHash: tokenHash,
			ExpiresAt:        expiresAt.UTC(),
			CreatedAt:        now,
			LastSeenAt:       now,
			IP:               strings.TrimSpace(ip),
			UserAgent:        strings.TrimSpace(userAgent),
		}
		if err := r.writeSessionNoLock(session); err != nil {
			return err
		}
		out = session
		return nil
	})
	return out, err
}

func (r *Repository) GetSessionWithAdminByTokenHash(ctx context.Context, tokenHash string) (Session, Admin, error) {
	var sessionOut Session
	var adminOut Admin
	err := r.withLock(ctx, func() error {
		now := time.Now().UTC()
		if err := r.purgeExpiredSessionsNoLock(now); err != nil {
			return err
		}
		sessions, err := r.loadSessionsNoLock()
		if err != nil {
			return err
		}
		for _, session := range sessions {
			if session.SessionTokenHash != tokenHash {
				continue
			}
			if !session.ExpiresAt.After(now) {
				return ErrNotFound
			}
			admin, err := r.loadAdminNoLock(session.AdminID)
			if err != nil {
				return err
			}
			sessionOut = session
			adminOut = admin
			return nil
		}
		return ErrNotFound
	})
	return sessionOut, adminOut, err
}

func (r *Repository) TouchSession(ctx context.Context, sessionID string) error {
	return r.withLock(ctx, func() error {
		session, err := r.loadSessionNoLock(sessionID)
		if err != nil {
			if IsNotFound(err) {
				return nil
			}
			return err
		}
		session.LastSeenAt = time.Now().UTC()
		return r.writeSessionNoLock(session)
	})
}

func (r *Repository) DeleteSessionByHash(ctx context.Context, tokenHash string) error {
	return r.withLock(ctx, func() error {
		sessions, err := r.loadSessionsNoLock()
		if err != nil {
			return err
		}
		for _, session := range sessions {
			if session.SessionTokenHash != tokenHash {
				continue
			}
			if err := os.Remove(sessionPath(r.sessionsDir, session.ID)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) loadAdminsNoLock() ([]Admin, error) { return loadEntities[Admin](r.adminsDir) }
func (r *Repository) loadAdminNoLock(id string) (Admin, error) { return loadEntity[Admin](adminPath(r.adminsDir, id)) }
func (r *Repository) writeAdminNoLock(admin Admin) error { return writeJSONFile(adminPath(r.adminsDir, admin.ID), 0o600, admin) }
func (r *Repository) loadSessionsNoLock() ([]Session, error) { return loadEntities[Session](r.sessionsDir) }
func (r *Repository) loadSessionNoLock(id string) (Session, error) { return loadEntity[Session](sessionPath(r.sessionsDir, id)) }
func (r *Repository) writeSessionNoLock(session Session) error { return writeJSONFile(sessionPath(r.sessionsDir, session.ID), 0o600, session) }

func (r *Repository) purgeExpiredSessionsNoLock(now time.Time) error {
	sessions, err := r.loadSessionsNoLock()
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if session.ExpiresAt.After(now) {
			continue
		}
		if err := os.Remove(sessionPath(r.sessionsDir, session.ID)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
