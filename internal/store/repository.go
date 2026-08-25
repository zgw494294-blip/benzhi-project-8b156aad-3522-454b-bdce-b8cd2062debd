package store

import (
	"context"
	"stone-restoration-trial/internal/domain"
)

type Repository interface {
	CreateCase(context.Context, *domain.RestorationCase, string, string, domain.AuditEvent) (*domain.RestorationCase, bool, error)
	UpdateCase(context.Context, string, int64, string, string, func(*domain.RestorationCase) ([]domain.AuditEvent, error)) (*domain.RestorationCase, bool, error)
	GetCase(context.Context, string) (*domain.RestorationCase, error)
	ListCases(context.Context) ([]domain.RestorationCase, error)
	Timeline(context.Context, string) ([]domain.AuditEvent, error)
	Approval(context.Context, string) (*domain.ApprovalRecord, error)
	Check(context.Context) error
	Close() error
}
