package workflow

import (
	"context"
	"stone-restoration-trial/internal/domain"
	"stone-restoration-trial/internal/store"
	"time"
)

type Service struct {
	repository store.Repository
	now        func() time.Time
}

func New(repository store.Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func NewWithClock(repository store.Repository, clock func() time.Time) *Service {
	return &Service{repository: repository, now: clock}
}

func (s *Service) GetCase(ctx context.Context, id string) (*domain.RestorationCase, error) {
	value, err := s.repository.GetCase(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := domain.PopulateDerivedEvidence(value); err != nil {
		return nil, err
	}
	return value, nil
}
func (s *Service) ListCases(ctx context.Context) ([]domain.RestorationCase, error) {
	values, err := s.repository.ListCases(ctx)
	if err != nil {
		return nil, err
	}
	for i := range values {
		if err := domain.PopulateDerivedEvidence(&values[i]); err != nil {
			return nil, err
		}
	}
	return values, nil
}
func (s *Service) Timeline(ctx context.Context, id string) ([]domain.AuditEvent, error) {
	return s.repository.Timeline(ctx, id)
}
func (s *Service) Approval(ctx context.Context, id string) (*domain.ApprovalRecord, error) {
	return s.repository.Approval(ctx, id)
}

func (s *Service) ApprovalEvidence(ctx context.Context, id string) (*domain.ApprovalEvidence, error) {
	record, err := s.repository.Approval(ctx, id)
	if err != nil {
		return nil, err
	}
	return domain.VerifyApprovalSnapshot(record)
}

func (s *Service) Health(ctx context.Context) error {
	return s.repository.Check(ctx)
}

type CommandMeta struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Actor           string `json:"actor"`
}

func event(caseID, eventType, summary, actor string, now time.Time, data map[string]any) domain.AuditEvent {
	return domain.AuditEvent{EventID: newID("evt"), CaseID: caseID, EventType: eventType, Summary: summary, Actor: actor, Data: data, OccurredAt: now.UTC()}
}
