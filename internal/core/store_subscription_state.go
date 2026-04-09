package core

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *Store) GetSubscriptionStateByUser(ctx context.Context, userID string) (Subscription, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx), `SELECT
		id,
		user_id,
		profile_name,
		enabled,
		COALESCE(primary_token_id, ''),
		COALESCE(artifact_version, 1),
		COALESCE(artifacts_need_refresh, 1),
		last_artifact_rendered_at_ns,
		last_artifact_refresh_reason,
		created_at_ns,
		updated_at_ns
	FROM core_subscriptions
	WHERE user_id = ?
	LIMIT 1`, normalizeString(userID))
	item, err := scanSubscriptionState(row)
	if err != nil {
		return Subscription{}, err
	}
	return item, nil
}

func (s *Store) GetSubscriptionState(ctx context.Context, id string) (Subscription, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx), `SELECT
		id,
		user_id,
		profile_name,
		enabled,
		COALESCE(primary_token_id, ''),
		COALESCE(artifact_version, 1),
		COALESCE(artifacts_need_refresh, 1),
		last_artifact_rendered_at_ns,
		last_artifact_refresh_reason,
		created_at_ns,
		updated_at_ns
	FROM core_subscriptions
	WHERE id = ?
	LIMIT 1`, normalizeString(id))
	item, err := scanSubscriptionState(row)
	if err != nil {
		return Subscription{}, err
	}
	return item, nil
}

func (s *Store) ListSubscriptionTokensState(ctx context.Context, subscriptionID string) ([]SubscriptionToken, error) {
	rows, err := s.db.QueryContext(resolveCtx(ctx), `SELECT
		id,
		subscription_id,
		token_prefix,
		COALESCE(is_primary, 0),
		revoked_at_ns,
		expires_at_ns,
		last_used_at_ns,
		last_used_ip,
		created_at_ns
	FROM core_subscription_tokens
	WHERE subscription_id = ?
	ORDER BY COALESCE(is_primary, 0) DESC, created_at_ns DESC`, normalizeString(subscriptionID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]SubscriptionToken, 0)
	for rows.Next() {
		item, err := scanSubscriptionTokenState(rows)
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

func (s *Store) GetSubscriptionTokenSecret(ctx context.Context, tokenID string) (SubscriptionToken, string, error) {
	row := s.db.QueryRowContext(resolveCtx(ctx), `SELECT
		id,
		subscription_id,
		token_prefix,
		COALESCE(is_primary, 0),
		revoked_at_ns,
		expires_at_ns,
		last_used_at_ns,
		last_used_ip,
		created_at_ns,
		COALESCE(token_plaintext_enc, '')
	FROM core_subscription_tokens
	WHERE id = ?
	LIMIT 1`, normalizeString(tokenID))
	return scanSubscriptionTokenSecret(row)
}

func (s *Store) UpdateSubscriptionTokenSecret(ctx context.Context, tokenID string, plaintextEnc string) error {
	result, err := s.db.ExecContext(resolveCtx(ctx), `UPDATE core_subscription_tokens SET token_plaintext_enc = ? WHERE id = ?`, nullIfEmpty(plaintextEnc), normalizeString(tokenID))
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateSubscriptionPrimaryToken(ctx context.Context, subscriptionID string, tokenID string) error {
	subscriptionID = normalizeString(subscriptionID)
	tokenID = normalizeString(tokenID)
	if subscriptionID == "" {
		return fmt.Errorf("subscription_id is required")
	}
	if tokenID == "" {
		return fmt.Errorf("token_id is required")
	}
	tx, err := s.db.BeginTx(resolveCtx(ctx), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(resolveCtx(ctx), `UPDATE core_subscription_tokens SET is_primary = CASE WHEN id = ? THEN 1 ELSE 0 END WHERE subscription_id = ?`, tokenID, subscriptionID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(resolveCtx(ctx), `UPDATE core_subscriptions SET primary_token_id = ?, updated_at_ns = ? WHERE id = ?`, tokenID, nowNano(), subscriptionID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) MarkSubscriptionArtifactsDirty(ctx context.Context, subscriptionID string, reason string) error {
	result, err := s.db.ExecContext(resolveCtx(ctx), `UPDATE core_subscriptions
	SET artifact_version = COALESCE(artifact_version, 0) + 1,
		artifacts_need_refresh = 1,
		last_artifact_refresh_reason = ?,
		updated_at_ns = ?
	WHERE id = ?`, nullIfEmpty(strings.TrimSpace(reason)), nowNano(), normalizeString(subscriptionID))
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MarkSubscriptionsArtifactsDirtyByUserIDs(ctx context.Context, userIDs []string, reason string) (int, error) {
	clean := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if trimmed := normalizeString(id); trimmed != "" {
			clean = append(clean, trimmed)
		}
	}
	if len(clean) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(clean))
	args := make([]any, 0, len(clean)+2)
	for i, id := range clean {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append([]any{nullIfEmpty(strings.TrimSpace(reason)), nowNano()}, args...)
	query := `UPDATE core_subscriptions
	SET artifact_version = COALESCE(artifact_version, 0) + 1,
		artifacts_need_refresh = 1,
		last_artifact_refresh_reason = ?,
		updated_at_ns = ?
	WHERE user_id IN (` + strings.Join(placeholders, ",") + `)`
	result, err := s.db.ExecContext(resolveCtx(ctx), query, args...)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(rows), nil
}

func (s *Store) MarkSubscriptionsArtifactsDirtyByServer(ctx context.Context, serverID string, reason string) (int, error) {
	result, err := s.db.ExecContext(resolveCtx(ctx), `UPDATE core_subscriptions
	SET artifact_version = COALESCE(artifact_version, 0) + 1,
		artifacts_need_refresh = 1,
		last_artifact_refresh_reason = ?,
		updated_at_ns = ?
	WHERE user_id IN (
		SELECT DISTINCT ua.user_id
		FROM core_user_access ua
		JOIN core_inbounds ib ON ib.id = ua.inbound_id
		WHERE ib.server_id = ?
	)`, nullIfEmpty(strings.TrimSpace(reason)), nowNano(), normalizeString(serverID))
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(rows), nil
}

func (s *Store) MarkSubscriptionsArtifactsDirtyByClientProfile(ctx context.Context, profileID string, reason string) (int, error) {
	result, err := s.db.ExecContext(resolveCtx(ctx), `UPDATE core_subscriptions
	SET artifact_version = COALESCE(artifact_version, 0) + 1,
		artifacts_need_refresh = 1,
		last_artifact_refresh_reason = ?,
		updated_at_ns = ?
	WHERE user_id IN (
		SELECT DISTINCT user_id
		FROM core_user_access
		WHERE client_profile_id = ?
	)`, nullIfEmpty(strings.TrimSpace(reason)), nowNano(), normalizeString(profileID))
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(rows), nil
}

func (s *Store) MarkSubscriptionArtifactsRendered(ctx context.Context, subscriptionID string) error {
	result, err := s.db.ExecContext(resolveCtx(ctx), `UPDATE core_subscriptions
	SET artifacts_need_refresh = 0,
		last_artifact_rendered_at_ns = ?,
		last_artifact_refresh_reason = NULL,
		updated_at_ns = ?
	WHERE id = ?`, nowNano(), nowNano(), normalizeString(subscriptionID))
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ResolveSubscriptionTokenState(ctx context.Context, plainToken string, ip string) (TokenContext, error) {
	plainToken = normalizeString(plainToken)
	if plainToken == "" {
		return TokenContext{}, ErrInvalidToken
	}
	prefix := tokenPrefix(plainToken)
	rows, err := s.db.QueryContext(
		resolveCtx(ctx),
		`SELECT
			t.id,
			t.subscription_id,
			t.token_prefix,
			COALESCE(t.is_primary, 0),
			t.token_hash,
			t.token_salt,
			t.revoked_at_ns,
			t.expires_at_ns,
			t.last_used_at_ns,
			t.last_used_ip,
			t.created_at_ns,
			s.id,
			s.user_id,
			s.profile_name,
			s.enabled,
			COALESCE(s.primary_token_id, ''),
			COALESCE(s.artifact_version, 1),
			COALESCE(s.artifacts_need_refresh, 1),
			s.last_artifact_rendered_at_ns,
			s.last_artifact_refresh_reason,
			s.created_at_ns,
			s.updated_at_ns,
			u.id,
			u.username,
			u.enabled,
			u.traffic_limit_bytes,
			u.traffic_used_up_bytes,
			u.traffic_used_down_bytes,
			u.expire_at_ns,
			u.created_at_ns,
			u.updated_at_ns
		FROM core_subscription_tokens t
		JOIN core_subscriptions s ON s.id = t.subscription_id
		JOIN core_users u ON u.id = s.user_id
		WHERE t.token_prefix = ?`,
		prefix,
	)
	if err != nil {
		return TokenContext{}, err
	}
	defer rows.Close()

	type tokenCandidate struct {
		TokenHash string
		TokenSalt string
		Ctx       TokenContext
	}

	candidates := make([]tokenCandidate, 0)
	for rows.Next() {
		var (
			ctxItem          TokenContext
			tokenHashValue   string
			tokenSaltValue   string
			tIsPrimary       int64
			tRevoked         sql.NullInt64
			tExpires         sql.NullInt64
			tLastUsed        sql.NullInt64
			tLastUsedIP      sql.NullString
			tCreated         int64
			sEnabled         int64
			sPrimaryTokenID  string
			sArtifactVersion int64
			sNeedRefresh     int64
			sLastRendered    sql.NullInt64
			sRefreshReason   sql.NullString
			sCreated         int64
			sUpdated         int64
			uEnabled         int64
			uExpire          sql.NullInt64
			uCreated         int64
			uUpdated         int64
		)
		if err := rows.Scan(
			&ctxItem.Token.ID,
			&ctxItem.Token.SubscriptionID,
			&ctxItem.Token.TokenPrefix,
			&tIsPrimary,
			&tokenHashValue,
			&tokenSaltValue,
			&tRevoked,
			&tExpires,
			&tLastUsed,
			&tLastUsedIP,
			&tCreated,
			&ctxItem.Subscription.ID,
			&ctxItem.Subscription.UserID,
			&ctxItem.Subscription.ProfileName,
			&sEnabled,
			&sPrimaryTokenID,
			&sArtifactVersion,
			&sNeedRefresh,
			&sLastRendered,
			&sRefreshReason,
			&sCreated,
			&sUpdated,
			&ctxItem.User.ID,
			&ctxItem.User.Username,
			&uEnabled,
			&ctxItem.User.TrafficLimitBytes,
			&ctxItem.User.TrafficUsedUpBytes,
			&ctxItem.User.TrafficUsedDownBytes,
			&uExpire,
			&uCreated,
			&uUpdated,
		); err != nil {
			return TokenContext{}, err
		}
		ctxItem.Token.IsPrimary = intToBool(tIsPrimary)
		ctxItem.Token.RevokedAt = optionalTime(tRevoked)
		ctxItem.Token.ExpiresAt = optionalTime(tExpires)
		ctxItem.Token.LastUsedAt = optionalTime(tLastUsed)
		ctxItem.Token.LastUsedIP = optionalString(tLastUsedIP)
		ctxItem.Token.CreatedAt = fromUnixNano(tCreated)
		ctxItem.Subscription.Enabled = intToBool(sEnabled)
		ctxItem.Subscription.PrimaryTokenID = strings.TrimSpace(sPrimaryTokenID)
		ctxItem.Subscription.ArtifactVersion = int(sArtifactVersion)
		ctxItem.Subscription.ArtifactsNeedRefresh = intToBool(sNeedRefresh)
		ctxItem.Subscription.LastArtifactRenderedAt = optionalTime(sLastRendered)
		ctxItem.Subscription.LastArtifactRefreshReason = optionalString(sRefreshReason)
		ctxItem.Subscription.CreatedAt = fromUnixNano(sCreated)
		ctxItem.Subscription.UpdatedAt = fromUnixNano(sUpdated)
		ctxItem.User.Enabled = intToBool(uEnabled)
		ctxItem.User.ExpireAt = optionalTime(uExpire)
		ctxItem.User.CreatedAt = fromUnixNano(uCreated)
		ctxItem.User.UpdatedAt = fromUnixNano(uUpdated)
		candidates = append(candidates, tokenCandidate{TokenHash: tokenHashValue, TokenSalt: tokenSaltValue, Ctx: ctxItem})
	}
	if err := rows.Err(); err != nil {
		return TokenContext{}, err
	}
	if len(candidates) == 0 {
		return TokenContext{}, ErrInvalidToken
	}

	computed := make([]tokenCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		candidateHash := tokenHash(candidate.TokenSalt, plainToken)
		if hashEqual(candidate.TokenHash, candidateHash) {
			computed = append(computed, candidate)
		}
	}
	if len(computed) == 0 {
		return TokenContext{}, ErrInvalidToken
	}
	sort.Slice(computed, func(i, j int) bool {
		if computed[i].Ctx.Token.IsPrimary != computed[j].Ctx.Token.IsPrimary {
			return computed[i].Ctx.Token.IsPrimary
		}
		return computed[i].Ctx.Token.CreatedAt.After(computed[j].Ctx.Token.CreatedAt)
	})
	selected := computed[0].Ctx
	if selected.Token.RevokedAt != nil {
		return TokenContext{}, ErrTokenRevoked
	}
	if selected.Token.ExpiresAt != nil && !selected.Token.ExpiresAt.After(time.Now().UTC()) {
		return TokenContext{}, ErrTokenRevoked
	}
	if _, err := s.db.ExecContext(resolveCtx(ctx), `UPDATE core_subscription_tokens SET last_used_at_ns = ?, last_used_ip = ? WHERE id = ?`, nowNano(), nullIfEmpty(ip), selected.Token.ID); err == nil {
		now := time.Now().UTC()
		selected.Token.LastUsedAt = &now
		trimmedIP := strings.TrimSpace(ip)
		if trimmedIP != "" {
			selected.Token.LastUsedIP = &trimmedIP
		}
	}
	return selected, nil
}

func scanSubscriptionState(row interface{ Scan(dest ...any) error }) (Subscription, error) {
	var (
		item          Subscription
		enabled       int64
		artifactVer   int64
		needRefresh   int64
		primaryToken  string
		renderedAt    sql.NullInt64
		refreshReason sql.NullString
		createdAt     int64
		updatedAt     int64
	)
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.ProfileName,
		&enabled,
		&primaryToken,
		&artifactVer,
		&needRefresh,
		&renderedAt,
		&refreshReason,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Subscription{}, parseUnique(err)
	}
	item.Enabled = intToBool(enabled)
	item.PrimaryTokenID = strings.TrimSpace(primaryToken)
	item.ArtifactVersion = int(artifactVer)
	item.ArtifactsNeedRefresh = intToBool(needRefresh)
	item.LastArtifactRenderedAt = optionalTime(renderedAt)
	item.LastArtifactRefreshReason = optionalString(refreshReason)
	item.CreatedAt = fromUnixNano(createdAt)
	item.UpdatedAt = fromUnixNano(updatedAt)
	return item, nil
}

func scanSubscriptionTokenState(row interface{ Scan(dest ...any) error }) (SubscriptionToken, error) {
	var (
		item       SubscriptionToken
		isPrimary  int64
		revokedAt  sql.NullInt64
		expiresAt  sql.NullInt64
		lastUsedAt sql.NullInt64
		lastUsedIP sql.NullString
		createdAt  int64
	)
	if err := row.Scan(
		&item.ID,
		&item.SubscriptionID,
		&item.TokenPrefix,
		&isPrimary,
		&revokedAt,
		&expiresAt,
		&lastUsedAt,
		&lastUsedIP,
		&createdAt,
	); err != nil {
		return SubscriptionToken{}, parseUnique(err)
	}
	item.IsPrimary = intToBool(isPrimary)
	item.RevokedAt = optionalTime(revokedAt)
	item.ExpiresAt = optionalTime(expiresAt)
	item.LastUsedAt = optionalTime(lastUsedAt)
	item.LastUsedIP = optionalString(lastUsedIP)
	item.CreatedAt = fromUnixNano(createdAt)
	return item, nil
}

func scanSubscriptionTokenSecret(row interface{ Scan(dest ...any) error }) (SubscriptionToken, string, error) {
	var (
		item       SubscriptionToken
		isPrimary  int64
		revokedAt  sql.NullInt64
		expiresAt  sql.NullInt64
		lastUsedAt sql.NullInt64
		lastUsedIP sql.NullString
		createdAt  int64
		secret     string
	)
	if err := row.Scan(
		&item.ID,
		&item.SubscriptionID,
		&item.TokenPrefix,
		&isPrimary,
		&revokedAt,
		&expiresAt,
		&lastUsedAt,
		&lastUsedIP,
		&createdAt,
		&secret,
	); err != nil {
		return SubscriptionToken{}, "", parseUnique(err)
	}
	item.IsPrimary = intToBool(isPrimary)
	item.RevokedAt = optionalTime(revokedAt)
	item.ExpiresAt = optionalTime(expiresAt)
	item.LastUsedAt = optionalTime(lastUsedAt)
	item.LastUsedIP = optionalString(lastUsedIP)
	item.CreatedAt = fromUnixNano(createdAt)
	return item, strings.TrimSpace(secret), nil
}
