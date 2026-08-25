package workflow

import (
	"context"
	"stone-restoration-trial/internal/domain"
	"stone-restoration-trial/internal/store"
	"sync"
	"time"
)

type Service struct {
	repository store.Repository
	now        func() time.Time
	caseMu     sync.Mutex
	caseLoads  map[string]*caseLoad
}

type caseLoad struct {
	done  chan struct{}
	value *domain.RestorationCase
	err   error
}

func New(repository store.Repository) *Service {
	return &Service{repository: repository, now: time.Now, caseLoads: make(map[string]*caseLoad)}
}

func NewWithClock(repository store.Repository, clock func() time.Time) *Service {
	return &Service{repository: repository, now: clock, caseLoads: make(map[string]*caseLoad)}
}

func (s *Service) GetCase(ctx context.Context, id string) (*domain.RestorationCase, error) {
	s.caseMu.Lock()
	if pending := s.caseLoads[id]; pending != nil {
		s.caseMu.Unlock()
		select {
		case <-pending.done:
			return pending.value, pending.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	pending := &caseLoad{done: make(chan struct{})}
	s.caseLoads[id] = pending
	s.caseMu.Unlock()

	value, err := s.repository.GetCase(ctx, id)
	if err == nil {
		err = domain.PopulateDerivedEvidence(value)
	}
	pending.value = value
	pending.err = err

	s.caseMu.Lock()
	delete(s.caseLoads, id)
	close(pending.done)
	s.caseMu.Unlock()
	return value, err
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
