package core

import "context"

func (s *Service) GetPolicyUsage(ctx context.Context, kind string, id string) (PolicyUsage, error) {
	return s.store.GetPolicyUsage(ctx, kind, id)
}
