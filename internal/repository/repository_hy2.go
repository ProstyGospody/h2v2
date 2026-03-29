package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	hysteriadomain "h2v2/internal/domain/hysteria"
)

func (r *Repository) CreateHysteriaUser(ctx context.Context, username string, password string, note *string, overrides *hysteriadomain.ClientOverrides) (HysteriaUser, error) {
	var out HysteriaUser
	err := r.withLock(ctx, func() error {
		normalizedUsername, err := hysteriadomain.NormalizeUsername(username)
		if err != nil {
			return err
		}
		normalizedPassword, err := hysteriadomain.NormalizePassword(password)
		if err != nil {
			return err
		}
		users, err := r.loadHysteriaUsersNoLock()
		if err != nil {
			return err
		}
		for _, user := range users {
			if user.UsernameNormalized == normalizedUsername {
				return ErrUniqueViolation
			}
		}
		now := time.Now().UTC()
		user := HysteriaUser{
			ID:                 uuid.NewString(),
			Username:           normalizedUsername,
			UsernameNormalized: normalizedUsername,
			Password:           normalizedPassword,
			Enabled:            true,
			Note:               hysteriadomain.NormalizeNote(note),
			ClientOverrides:    hysteriadomain.NormalizeClientOverrides(overrides),
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if err := r.writeHysteriaUserNoLock(user); err != nil {
			return err
		}
		out = user
		return nil
	})
	return out, err
}

func (r *Repository) ListHysteriaUsers(ctx context.Context, limit int, offset int) ([]HysteriaUserView, error) {
	var out []HysteriaUserView
	err := r.withLock(ctx, func() error {
		users, err := r.loadHysteriaUsersNoLock()
		if err != nil {
			return err
		}
		sort.Slice(users, func(i, j int) bool { return users[i].CreatedAt.After(users[j].CreatedAt) })
		items := make([]HysteriaUserView, 0, len(users))
		for _, user := range users {
			items = append(items, r.hysteriaUserViewNoLock(user))
		}
		out = paginate(items, limit, offset)
		return nil
	})
	return out, err
}

func (r *Repository) ListEnabledHysteriaUsers(ctx context.Context) ([]HysteriaUser, error) {
	var out []HysteriaUser
	err := r.withLock(ctx, func() error {
		users, err := r.loadHysteriaUsersNoLock()
		if err != nil {
			return err
		}
		for _, user := range users {
			if user.Enabled {
				out = append(out, user)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
		return nil
	})
	return out, err
}

func (r *Repository) GetHysteriaUser(ctx context.Context, id string) (HysteriaUserView, error) {
	var out HysteriaUserView
	err := r.withLock(ctx, func() error {
		user, err := r.loadHysteriaUserNoLock(id)
		if err != nil {
			return err
		}
		out = r.hysteriaUserViewNoLock(user)
		return nil
	})
	return out, err
}

func (r *Repository) UpdateHysteriaUser(ctx context.Context, id string, username string, password string, note *string, overrides *hysteriadomain.ClientOverrides) (HysteriaUserView, error) {
	var out HysteriaUserView
	err := r.withLock(ctx, func() error {
		current, err := r.loadHysteriaUserNoLock(id)
		if err != nil {
			return err
		}
		normalizedUsername, err := hysteriadomain.NormalizeUsername(username)
		if err != nil {
			return err
		}
		normalizedPassword, err := hysteriadomain.NormalizePassword(password)
		if err != nil {
			return err
		}
		users, err := r.loadHysteriaUsersNoLock()
		if err != nil {
			return err
		}
		for _, user := range users {
			if user.ID == id {
				continue
			}
			if user.UsernameNormalized == normalizedUsername {
				return ErrUniqueViolation
			}
		}
		current.Username = normalizedUsername
		current.UsernameNormalized = normalizedUsername
		current.Password = normalizedPassword
		current.Note = hysteriadomain.NormalizeNote(note)
		current.ClientOverrides = hysteriadomain.NormalizeClientOverrides(overrides)
		current.UpdatedAt = time.Now().UTC()
		if err := r.writeHysteriaUserNoLock(current); err != nil {
			return err
		}
		out = r.hysteriaUserViewNoLock(current)
		return nil
	})
	return out, err
}
func (r *Repository) DeleteHysteriaUser(ctx context.Context, id string) error {
	return r.withLock(ctx, func() error {
		if _, err := r.loadHysteriaUserNoLock(id); err != nil {
			return err
		}
		if err := os.Remove(hysteriaUserPath(r.hysteriaUsersDir, id)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.RemoveAll(filepath.Join(r.hysteriaSnapshotsDir, id)); err != nil {
			return err
		}
		return nil
	})
}

func (r *Repository) SetHysteriaUserEnabled(ctx context.Context, id string, enabled bool) error {
	return r.withLock(ctx, func() error {
		user, err := r.loadHysteriaUserNoLock(id)
		if err != nil {
			return err
		}
		user.Enabled = enabled
		user.UpdatedAt = time.Now().UTC()
		return r.writeHysteriaUserNoLock(user)
	})
}

func (r *Repository) TouchHysteriaUserLastSeen(ctx context.Context, id string, seenAt time.Time) error {
	return r.withLock(ctx, func() error {
		user, err := r.loadHysteriaUserNoLock(id)
		if err != nil {
			if IsNotFound(err) {
				return nil
			}
			return err
		}
		ts := seenAt.UTC()
		user.LastSeenAt = &ts
		return r.writeHysteriaUserNoLock(user)
	})
}

func (r *Repository) InsertHysteriaSnapshots(ctx context.Context, snapshots []HysteriaSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	return r.withLock(ctx, func() error {
		meta, err := r.loadMetaNoLock()
		if err != nil {
			return err
		}
		for idx := range snapshots {
			meta.NextHy2SnapshotID++
			snapshots[idx].ID = meta.NextHy2SnapshotID
			if snapshots[idx].SnapshotAt.IsZero() {
				snapshots[idx].SnapshotAt = time.Now().UTC()
			} else {
				snapshots[idx].SnapshotAt = snapshots[idx].SnapshotAt.UTC()
			}
		}
		if err := r.saveMetaNoLock(meta); err != nil {
			return err
		}
		for _, snapshot := range snapshots {
			if err := r.writeHysteriaSnapshotNoLock(snapshot); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) GetHysteriaStatsOverview(ctx context.Context) (HysteriaOverview, error) {
	var out HysteriaOverview
	err := r.withLock(ctx, func() error {
		users, err := r.loadHysteriaUsersNoLock()
		if err != nil {
			return err
		}
		for _, user := range users {
			if user.Enabled {
				out.EnabledUsers++
			}
			snapshot, ok, err := r.latestHysteriaSnapshotNoLock(user.ID)
			if err != nil {
				return err
			}
			if ok {
				out.TotalTxBytes += snapshot.TxBytes
				out.TotalRxBytes += snapshot.RxBytes
				out.OnlineCount += int64(snapshot.Online)
			}
		}
		return nil
	})
	return out, err
}

func (r *Repository) ListHysteriaSnapshots(ctx context.Context, userID string, limit int, offset int) ([]HysteriaSnapshot, error) {
	var out []HysteriaSnapshot
	err := r.withLock(ctx, func() error {
		items, err := r.loadHysteriaSnapshotsNoLock(strings.TrimSpace(userID))
		if err != nil {
			return err
		}
		sort.Slice(items, func(i, j int) bool { return items[i].SnapshotAt.After(items[j].SnapshotAt) })
		out = paginate(items, limit, offset)
		return nil
	})
	return out, err
}

func (r *Repository) loadHysteriaUsersNoLock() ([]HysteriaUser, error) {
	return loadEntities[HysteriaUser](r.hysteriaUsersDir)
}

func (r *Repository) loadHysteriaUserNoLock(id string) (HysteriaUser, error) {
	return loadEntity[HysteriaUser](hysteriaUserPath(r.hysteriaUsersDir, id))
}

func (r *Repository) writeHysteriaUserNoLock(user HysteriaUser) error {
	return writeJSONFile(hysteriaUserPath(r.hysteriaUsersDir, user.ID), 0o600, user)
}

func (r *Repository) latestHysteriaSnapshotNoLock(userID string) (HysteriaSnapshot, bool, error) {
	items, err := r.loadHysteriaSnapshotsNoLock(userID)
	if err != nil {
		return HysteriaSnapshot{}, false, err
	}
	if len(items) == 0 {
		return HysteriaSnapshot{}, false, nil
	}
	sort.Slice(items, func(i, j int) bool { return items[i].SnapshotAt.After(items[j].SnapshotAt) })
	return items[0], true, nil
}

func (r *Repository) loadHysteriaSnapshotsNoLock(userID string) ([]HysteriaSnapshot, error) {
	if userID != "" {
		return loadEntities[HysteriaSnapshot](filepath.Join(r.hysteriaSnapshotsDir, userID))
	}
	userDirs, err := os.ReadDir(r.hysteriaSnapshotsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]HysteriaSnapshot, 0)
	for _, entry := range userDirs {
		if !entry.IsDir() {
			continue
		}
		items, err := loadEntities[HysteriaSnapshot](filepath.Join(r.hysteriaSnapshotsDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	return out, nil
}

func (r *Repository) writeHysteriaSnapshotNoLock(snapshot HysteriaSnapshot) error {
	dir := filepath.Join(r.hysteriaSnapshotsDir, snapshot.UserID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(dir, numericJSONFile(snapshot.ID)), 0o600, snapshot)
}

func (r *Repository) hysteriaUserViewNoLock(user HysteriaUser) HysteriaUserView {
	item := HysteriaUserView{User: user}
	if latest, ok, err := r.latestHysteriaSnapshotNoLock(user.ID); err == nil && ok {
		item.LastTxBytes = latest.TxBytes
		item.LastRxBytes = latest.RxBytes
		item.OnlineCount = latest.Online
	}
	return item
}


