package core

import (
	"context"
	"strings"
	"time"
)

func (s *Service) GetDraftRevisionState(ctx context.Context, serverID string) (DraftRevisionState, error) {
	state := DraftRevisionState{ServerID: strings.TrimSpace(serverID)}
	if state.ServerID == "" {
		return state, nil
	}
	if current, err := s.store.GetCurrentConfigRevision(ctx, state.ServerID); err == nil {
		state.CurrentRevisionID = current.ID
		state.CurrentRevisionNo = current.RevisionNo
	}
	if latest, err := s.store.GetLatestConfigRevision(ctx, state.ServerID); err == nil {
		state.DraftRevisionID = latest.ID
		state.DraftRevisionNo = latest.RevisionNo
		state.PendingChanges = state.CurrentRevisionID == "" || latest.ID != state.CurrentRevisionID
		state.CheckOK = latest.CheckOK
		state.CheckError = latest.CheckError
		state.ApplyStatus = latest.ApplyStatus
		state.ApplyError = latest.ApplyError
	}
	return state, nil
}

func (s *Service) renderDraftRevisionStates(ctx context.Context, serverIDs []string) ([]DraftRevisionState, error) {
	unique := uniqueNonEmptyStrings(serverIDs)
	states := make([]DraftRevisionState, 0, len(unique))
	for _, serverID := range unique {
		if _, err := s.RenderServerConfig(ctx, serverID, nil); err != nil {
			return nil, err
		}
		state, err := s.GetDraftRevisionState(ctx, serverID)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, nil
}

func (s *Service) PreviewBulkUsers(ctx context.Context, patch BulkUserPatch) (ChangeImpact, error) {
	impact, err := s.collectUserImpact(ctx, patch.IDs)
	if err != nil {
		return ChangeImpact{}, err
	}
	if patch.InboundID != nil {
		targetInboundID := strings.TrimSpace(*patch.InboundID)
		if targetInboundID != "" {
			impact.InboundIDs = uniqueNonEmptyStrings(append(impact.InboundIDs, targetInboundID))
			impact.AffectedInbounds = len(impact.InboundIDs)
			if inbound, err := s.store.GetInbound(ctx, targetInboundID); err == nil {
				impact.ServerIDs = uniqueNonEmptyStrings(append(impact.ServerIDs, inbound.ServerID))
			}
		}
	}
	impact.RequiresRuntimeApply = patch.DeleteMode != BulkDeleteModeNone || patch.Enabled != nil || patch.ExtendSeconds != 0 || patch.SetExpireAt != nil || patch.ClearExpire || patch.TrafficLimitBytes != nil || patch.InboundID != nil
	impact.RequiresArtifactRefresh = impact.RequiresRuntimeApply || patch.ClientProfileID != nil || patch.RotateTokens || patch.RegenerateArtifacts
	return impact, nil
}

func (s *Service) ApplyBulkUsers(ctx context.Context, patch BulkUserPatch) (BulkMutationResult, error) {
	impact, err := s.PreviewBulkUsers(ctx, patch)
	if err != nil {
		return BulkMutationResult{}, err
	}
	usersByID, err := s.store.ListUsersByIDs(ctx, patch.IDs)
	if err != nil {
		return BulkMutationResult{}, err
	}
	result := BulkMutationResult{Impact: impact}
	for _, userID := range uniqueNonEmptyStrings(patch.IDs) {
		user, ok := usersByID[userID]
		if !ok {
			continue
		}
		if patch.DeleteMode == BulkDeleteModeHard {
			if err := s.store.DeleteUser(ctx, user.ID); err != nil {
				return BulkMutationResult{}, err
			}
			result.Deleted++
			continue
		}
		if patch.DeleteMode == BulkDeleteModeSoft {
			user.Enabled = false
		}
		if patch.Enabled != nil {
			user.Enabled = *patch.Enabled
		}
		if patch.TrafficLimitBytes != nil {
			user.TrafficLimitBytes = *patch.TrafficLimitBytes
		}
		if patch.ClearExpire {
			user.ExpireAt = nil
		} else if patch.SetExpireAt != nil {
			ts := patch.SetExpireAt.UTC()
			user.ExpireAt = &ts
		} else if patch.ExtendSeconds != 0 {
			base := time.Now().UTC()
			if user.ExpireAt != nil && user.ExpireAt.After(base) {
				base = user.ExpireAt.UTC()
			}
			extended := base.Add(time.Duration(patch.ExtendSeconds) * time.Second)
			user.ExpireAt = &extended
		}
		if _, err := s.UpsertUser(ctx, user); err != nil {
			return BulkMutationResult{}, err
		}
		result.Updated++

		accesses, err := s.store.ListUserAccessByUser(ctx, user.ID)
		if err != nil {
			return BulkMutationResult{}, err
		}
		for _, access := range accesses {
			updatedAccess := access
			changed := false
			if patch.ClientProfileID != nil {
				updatedAccess.ClientProfileID = strings.TrimSpace(*patch.ClientProfileID)
				changed = true
			}
			if patch.InboundID != nil {
				targetInboundID := strings.TrimSpace(*patch.InboundID)
				if targetInboundID != "" && targetInboundID != updatedAccess.InboundID {
					updatedAccess.InboundID = targetInboundID
					changed = true
				}
			}
			if changed {
				if _, err := s.UpsertUserAccess(ctx, updatedAccess); err != nil {
					return BulkMutationResult{}, err
				}
			}
		}
		if patch.RotateTokens {
			if _, _, err := s.RotateSubscriptionTokenByUser(ctx, user.ID, nil); err != nil {
				return BulkMutationResult{}, err
			}
			result.Rotated++
		}
		if patch.RegenerateArtifacts {
			if sub, err := s.EnsureSubscriptionForUser(ctx, user.ID); err == nil {
				if err := s.store.MarkSubscriptionArtifactsDirty(ctx, sub.ID, "user_bulk_regenerated"); err == nil {
					result.Regenerated++
				}
			}
		}
	}
	if impact.RequiresRuntimeApply {
		drafts, err := s.renderDraftRevisionStates(ctx, impact.ServerIDs)
		if err != nil {
			return BulkMutationResult{}, err
		}
		result.Drafts = drafts
	}
	return result, nil
}

func (s *Service) PreviewBulkAccess(ctx context.Context, patch BulkAccessPatch) (ChangeImpact, error) {
	impact, err := s.collectAccessImpact(ctx, patch.IDs)
	if err != nil {
		return ChangeImpact{}, err
	}
	if patch.InboundID != nil {
		targetInboundID := strings.TrimSpace(*patch.InboundID)
		if targetInboundID != "" {
			impact.InboundIDs = uniqueNonEmptyStrings(append(impact.InboundIDs, targetInboundID))
			impact.AffectedInbounds = len(impact.InboundIDs)
			if inbound, err := s.store.GetInbound(ctx, targetInboundID); err == nil {
				impact.ServerIDs = uniqueNonEmptyStrings(append(impact.ServerIDs, inbound.ServerID))
			}
		}
	}
	impact.RequiresRuntimeApply = patch.DeleteMode != BulkDeleteModeNone || patch.Enabled != nil || patch.ExtendSeconds != 0 || patch.SetExpireAt != nil || patch.ClearExpire || patch.TrafficLimitBytes != nil || patch.InboundID != nil || patch.RotateCredentials
	impact.RequiresArtifactRefresh = impact.RequiresRuntimeApply || patch.ClientProfileID != nil || patch.RegenerateArtifacts
	return impact, nil
}

func (s *Service) ApplyBulkAccess(ctx context.Context, patch BulkAccessPatch) (BulkMutationResult, error) {
	impact, err := s.PreviewBulkAccess(ctx, patch)
	if err != nil {
		return BulkMutationResult{}, err
	}
	accesses, err := s.store.ListUserAccessByIDs(ctx, patch.IDs)
	if err != nil {
		return BulkMutationResult{}, err
	}
	result := BulkMutationResult{Impact: impact}
	for _, access := range accesses {
		if patch.DeleteMode == BulkDeleteModeHard {
			if err := s.DeleteUserAccess(ctx, access.ID); err != nil {
				return BulkMutationResult{}, err
			}
			result.Deleted++
			continue
		}
		updatedAccess := access
		if patch.DeleteMode == BulkDeleteModeSoft {
			updatedAccess.Enabled = false
		}
		if patch.Enabled != nil {
			updatedAccess.Enabled = *patch.Enabled
		}
		if patch.ClearExpire {
			updatedAccess.ExpireAtOverride = nil
		} else if patch.SetExpireAt != nil {
			ts := patch.SetExpireAt.UTC()
			updatedAccess.ExpireAtOverride = &ts
		} else if patch.ExtendSeconds != 0 {
			base := time.Now().UTC()
			if updatedAccess.ExpireAtOverride != nil && updatedAccess.ExpireAtOverride.After(base) {
				base = updatedAccess.ExpireAtOverride.UTC()
			}
			extended := base.Add(time.Duration(patch.ExtendSeconds) * time.Second)
			updatedAccess.ExpireAtOverride = &extended
		}
		if patch.TrafficLimitBytes != nil {
			limit := *patch.TrafficLimitBytes
			updatedAccess.TrafficLimitBytesOverride = &limit
		}
		if patch.ClientProfileID != nil {
			updatedAccess.ClientProfileID = strings.TrimSpace(*patch.ClientProfileID)
		}
		if patch.InboundID != nil {
			targetInboundID := strings.TrimSpace(*patch.InboundID)
			if targetInboundID != "" {
				updatedAccess.InboundID = targetInboundID
			}
		}
		if patch.RotateCredentials {
			if inbound, err := s.store.GetInbound(ctx, updatedAccess.InboundID); err == nil {
				if inbound.Protocol == InboundProtocolVLESS {
					updatedAccess.VLESSUUID = ""
				} else {
					updatedAccess.Hysteria2Password = ""
				}
			}
		}
		if _, err := s.UpsertUserAccess(ctx, updatedAccess); err != nil {
			return BulkMutationResult{}, err
		}
		result.Updated++
		if patch.RotateCredentials {
			result.Rotated++
		}
		if patch.RegenerateArtifacts {
			if sub, err := s.EnsureSubscriptionForUser(ctx, updatedAccess.UserID); err == nil {
				if err := s.store.MarkSubscriptionArtifactsDirty(ctx, sub.ID, "access_bulk_regenerated"); err == nil {
					result.Regenerated++
				}
			}
		}
	}
	if impact.RequiresRuntimeApply {
		drafts, err := s.renderDraftRevisionStates(ctx, impact.ServerIDs)
		if err != nil {
			return BulkMutationResult{}, err
		}
		result.Drafts = drafts
	}
	return result, nil
}

func (s *Service) collectUserImpact(ctx context.Context, userIDs []string) (ChangeImpact, error) {
	usersByID, err := s.store.ListUsersByIDs(ctx, userIDs)
	if err != nil {
		return ChangeImpact{}, err
	}
	impact := ChangeImpact{AffectedUsers: len(usersByID)}
	inboundIDs := make([]string, 0)
	serverIDs := make([]string, 0)
	subscriptionCount := 0
	for userID := range usersByID {
		accesses, err := s.store.ListUserAccessByUser(ctx, userID)
		if err != nil {
			return ChangeImpact{}, err
		}
		impact.AffectedAccess += len(accesses)
		for _, access := range accesses {
			inboundIDs = append(inboundIDs, access.InboundID)
			if inbound, err := s.store.GetInbound(ctx, access.InboundID); err == nil {
				serverIDs = append(serverIDs, inbound.ServerID)
			}
		}
		if _, err := s.store.GetSubscriptionStateByUser(ctx, userID); err == nil {
			subscriptionCount++
		}
	}
	impact.InboundIDs = uniqueNonEmptyStrings(inboundIDs)
	impact.ServerIDs = uniqueNonEmptyStrings(serverIDs)
	impact.AffectedInbounds = len(impact.InboundIDs)
	impact.AffectedSubscriptions = subscriptionCount
	impact.AffectedArtifacts = subscriptionCount
	return impact, nil
}

func (s *Service) collectAccessImpact(ctx context.Context, accessIDs []string) (ChangeImpact, error) {
	accesses, err := s.store.ListUserAccessByIDs(ctx, accessIDs)
	if err != nil {
		return ChangeImpact{}, err
	}
	impact := ChangeImpact{AffectedAccess: len(accesses)}
	userIDs := make([]string, 0, len(accesses))
	inboundIDs := make([]string, 0, len(accesses))
	serverIDs := make([]string, 0, len(accesses))
	for _, access := range accesses {
		userIDs = append(userIDs, access.UserID)
		inboundIDs = append(inboundIDs, access.InboundID)
		if inbound, err := s.store.GetInbound(ctx, access.InboundID); err == nil {
			serverIDs = append(serverIDs, inbound.ServerID)
		}
	}
	impact.AffectedUsers = len(uniqueNonEmptyStrings(userIDs))
	impact.InboundIDs = uniqueNonEmptyStrings(inboundIDs)
	impact.ServerIDs = uniqueNonEmptyStrings(serverIDs)
	impact.AffectedInbounds = len(impact.InboundIDs)
	subscriptionCount := 0
	for _, userID := range uniqueNonEmptyStrings(userIDs) {
		if _, err := s.store.GetSubscriptionStateByUser(ctx, userID); err == nil {
			subscriptionCount++
		}
	}
	impact.AffectedSubscriptions = subscriptionCount
	impact.AffectedArtifacts = subscriptionCount
	return impact, nil
}

func uniqueNonEmptyStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, raw := range items {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}
