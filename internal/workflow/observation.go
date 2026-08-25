package workflow

import (
	"context"
	"stone-restoration-trial/internal/domain"
	"time"
)

type RecordObservationCommand struct {
	CommandMeta
	Stage            domain.CuringStage `json:"stage"`
	ColorDifference  float64            `json:"colorDifference"`
	WaterAbsorption  float64            `json:"waterAbsorption"`
	AdhesionStrength float64            `json:"adhesionStrength"`
	SurfaceDefects   []string           `json:"surfaceDefects"`
	ObservedAt       time.Time          `json:"observedAt"`
	EvidenceSummary  string             `json:"evidenceSummary"`
}

func (s *Service) RecordObservation(ctx context.Context, caseID, patchID string, command RecordObservationCommand) (*domain.RestorationCase, bool, error) {
	if err := validateMeta(command.CommandMeta); err != nil {
		return nil, false, err
	}
	now := s.now().UTC()
	return s.updateCase(ctx, caseID, command.ExpectedVersion, command.IdempotencyKey, requestHash(command), func(c *domain.RestorationCase) ([]domain.AuditEvent, error) {
		if err := c.EnsureMutable(); err != nil {
			return nil, err
		}
		if c.Status != domain.StatusTesting && c.Status != domain.StatusRemediation {
			return nil, domain.ErrInvalidState
		}
		patch, err := c.Patch(patchID)
		if err != nil {
			return nil, err
		}
		if err := domain.ValidateNextStage(patch, command.Stage); err != nil {
			return nil, err
		}
		observation := domain.Observation{ObservationID: newID("obs"), PatchID: patchID, Stage: command.Stage, ColorDifference: command.ColorDifference, WaterAbsorption: command.WaterAbsorption, AdhesionStrength: command.AdhesionStrength, SurfaceDefects: command.SurfaceDefects, ObservedAt: command.ObservedAt.UTC(), EvidenceSummary: command.EvidenceSummary, RecordedBy: command.Actor}
		if err := domain.ValidateObservation(observation, now); err != nil {
			return nil, err
		}
		if err := domain.ValidateObservationTimeline(patch, observation); err != nil {
			return nil, err
		}
		patch.Observations = append(patch.Observations, observation)
		patch.CuringStage = observation.Stage
		patch.Trends = domain.BuildStageTrends(*patch, c.AcceptanceThresholds)
		latestTrend := patch.Trends[len(patch.Trends)-1]
		warningCodes := make([]string, 0, len(latestTrend.Warnings))
		for _, warning := range latestTrend.Warnings {
			warningCodes = append(warningCodes, warning.Code)
		}
		events := []domain.AuditEvent{event(caseID, "observation.recorded", "记录养护阶段观测并计算指标趋势", command.Actor, now, map[string]any{"patchID": patchID, "stage": command.Stage, "observationID": observation.ObservationID, "trend": latestTrend, "trendWarnings": warningCodes})}
		if command.Stage == domain.StageFinal {
			seeds := domain.EvaluateObservation(observation, c.AcceptanceThresholds)
			domain.ApplyFinalEvaluation(patch, observation, seeds, now)
			for _, seed := range seeds {
				d := domain.Deviation{DeviationID: newID("dev"), CaseID: caseID, PatchID: patchID, Metric: seed.Metric, MeasuredValue: seed.MeasuredValue, Threshold: seed.Threshold, Severity: seed.Severity, Status: domain.DeviationOpen}
				c.Deviations = append(c.Deviations, d)
			}
			if len(seeds) > 0 && c.Status == domain.StatusTesting {
				if err := domain.Transition(c, domain.StatusRemediation); err != nil {
					return nil, err
				}
			}
			if len(seeds) == 0 && patch.ParentPatchID != "" {
				closedIDs := closeRetestDeviationChain(c, patch, now)
				rootID, round, _ := domain.RetestTrace(c, patch.PatchID)
				events = append(events, event(caseID, "retest.passed", "后代复验合格并级联关闭祖先偏差", command.Actor, now, map[string]any{"patchID": patchID, "parentPatchID": patch.ParentPatchID, "rootPatchID": rootID, "retestRound": round, "closedDeviationIDs": closedIDs}))
			} else if len(seeds) > 0 && patch.ParentPatchID != "" {
				rootID, round, _ := domain.RetestTrace(c, patch.PatchID)
				events = append(events, event(caseID, "retest.failed", "复验终期不合格，保留祖先偏差链", command.Actor, now, map[string]any{"patchID": patchID, "rootPatchID": rootID, "retestRound": round, "deviationCount": len(seeds)}))
			}
			events = append(events, event(caseID, "patch.evaluated", "完成终期评估", command.Actor, now, map[string]any{"patchID": patchID, "conclusion": patch.EvaluationResult.Conclusion, "deviationCount": len(seeds)}))
		}
		c.Touch(now)
		return events, nil
	})
}

func closeRetestDeviationChain(c *domain.RestorationCase, patch *domain.TrialPatch, now time.Time) []string {
	closedIDs := []string{}
	childID, parentID := patch.PatchID, patch.ParentPatchID
	for parentID != "" {
		for i := range c.Deviations {
			d := &c.Deviations[i]
			if d.PatchID == parentID && d.RetestPatchID == childID && d.Status == domain.DeviationRemediated {
				d.Status = domain.DeviationClosed
				closed := now.UTC()
				d.ClosedAt = &closed
				closedIDs = append(closedIDs, d.DeviationID)
			}
		}
		parent, err := c.Patch(parentID)
		if err != nil {
			break
		}
		childID, parentID = parent.PatchID, parent.ParentPatchID
	}
	return closedIDs
}
