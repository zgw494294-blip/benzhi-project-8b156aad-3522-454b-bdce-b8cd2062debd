package domain

func Transition(c *RestorationCase, target CaseStatus) error {
	if err := c.EnsureMutable(); err != nil {
		return err
	}
	allowed := false
	switch c.Status {
	case StatusDraft:
		allowed = target == StatusTesting
	case StatusTesting:
		allowed = target == StatusRemediation || target == StatusPending
	case StatusRemediation:
		allowed = target == StatusPending
	case StatusPending:
		allowed = target == StatusRemediation || target == StatusApproved
	}
	if !allowed {
		return ErrInvalidState
	}
	c.Status = target
	return nil
}

func ValidateNextStage(patch *TrialPatch, stage CuringStage) error {
	next := 0
	if len(patch.Observations) > 0 {
		next = StageIndex(patch.Observations[len(patch.Observations)-1].Stage) + 1
	}
	if StageIndex(stage) != next {
		return Invalid("stage", "必须按初期、稳定期、终期顺序提交观测")
	}
	return nil
}
