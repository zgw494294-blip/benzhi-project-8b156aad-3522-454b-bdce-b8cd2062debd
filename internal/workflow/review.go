package workflow

import (
	"context"
	"fmt"
	"stone-restoration-trial/internal/domain"
	"strings"
)

type SubmitReviewCommand struct{ CommandMeta }

func (s *Service) SubmitReview(ctx context.Context, caseID string, command SubmitReviewCommand) (*domain.RestorationCase, bool, error) {
	if err := validateMeta(command.CommandMeta); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	return s.repository.UpdateCase(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, requestHash(command), func(c *domain.RestorationCase) ([]domain.AuditEvent, error) {
		if err := domain.CanSubmitForReview(c); err != nil {
			return nil, err
		}
		if err := domain.Transition(c, domain.StatusPending); err != nil {
			return nil, err
		}
		c.Touch(now)
		return []domain.AuditEvent{event(caseID, "review.submitted", "提交技术审查", command.Actor, now, map[string]any{"version": c.Version})}, nil
	})
}

type ReviewDecisionCommand struct {
	CommandMeta
	Decision          domain.Decision `json:"decision"`
	ReviewComment     string          `json:"reviewComment"`
	ApprovedFormulaID string          `json:"approvedFormulaID,omitempty"`
}

func (s *Service) DecideReview(ctx context.Context, caseID string, command ReviewDecisionCommand) (*domain.RestorationCase, bool, error) {
	if err := validateMeta(command.CommandMeta); err != nil {
		return nil, false, err
	}
	if command.Decision != domain.DecisionApprove && command.Decision != domain.DecisionReturn {
		return nil, false, domain.Invalid("decision", "审查结论必须为 approve 或 return")
	}
	if strings.TrimSpace(command.ReviewComment) == "" {
		return nil, false, domain.Invalid("reviewComment", "审查意见不能为空")
	}
	now := s.now().UTC()
	value, replayed, err := s.repository.UpdateCase(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, requestHash(command), func(c *domain.RestorationCase) ([]domain.AuditEvent, error) {
		if c.Status != domain.StatusPending {
			return nil, domain.ErrInvalidState
		}
		if command.Decision == domain.DecisionReturn {
			if err := domain.Transition(c, domain.StatusRemediation); err != nil {
				return nil, err
			}
			c.Touch(now)
			return []domain.AuditEvent{event(caseID, "review.returned", "技术审查退回整改", command.Actor, now, map[string]any{"comment": command.ReviewComment})}, nil
		}
		snapshot, digest, err := domain.BuildSnapshot(c, command.ApprovedFormulaID)
		if err != nil {
			return nil, err
		}
		if err := domain.Transition(c, domain.StatusApproved); err != nil {
			return nil, err
		}
		c.Touch(now)
		approval := &domain.ApprovalRecord{ApprovalID: fmt.Sprintf("APR-%s-%06d", now.Format("20060102"), now.UnixNano()%1000000), CaseID: caseID, Decision: domain.DecisionApprove, ReviewComment: strings.TrimSpace(command.ReviewComment), ApprovedFormulaID: command.ApprovedFormulaID, FrozenCaseVersion: c.Version, SnapshotDigest: digest, SnapshotJSON: snapshot, DecidedBy: command.Actor, DecidedAt: now}
		c.Approval = approval
		return []domain.AuditEvent{event(caseID, "review.approved", "技术审查批准并冻结证据", command.Actor, now, map[string]any{"approvalID": approval.ApprovalID, "formulaID": approval.ApprovedFormulaID, "snapshotDigest": digest})}, nil
	})
	if err != nil {
		return nil, false, err
	}
	delete(s.approvalCache, caseID)
	return value, replayed, nil
}
