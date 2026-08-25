package workflow

import (
	"context"
	"errors"
	"stone-restoration-trial/internal/domain"
)

type caseMutation func(*domain.RestorationCase) ([]domain.AuditEvent, error)

func (s *Service) updateCase(ctx context.Context, caseID string, expectedVersion int64, key, hash string, mutate caseMutation) (*domain.RestorationCase, bool, error) {
	value, replayed, err := s.repository.UpdateCase(ctx, caseID, expectedVersion, key, hash, mutate)
	if !errors.Is(err, domain.ErrConflict) {
		return value, replayed, err
	}
	latest, loadErr := s.repository.GetCase(ctx, caseID)
	if loadErr != nil {
		return nil, false, loadErr
	}
	if latest.Version <= expectedVersion {
		return value, replayed, err
	}
	return s.repository.UpdateCase(ctx, caseID, latest.Version, key, hash, mutate)
}
