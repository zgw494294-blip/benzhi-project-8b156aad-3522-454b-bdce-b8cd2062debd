package domain

import "time"

type RestorationCase struct {
	CaseID               string            `json:"caseID"`
	SiteName             string            `json:"siteName"`
	BuildingArea         string            `json:"buildingArea"`
	StoneType            string            `json:"stoneType"`
	DeteriorationSummary string            `json:"deteriorationSummary"`
	TargetAppearance     string            `json:"targetAppearance"`
	AcceptanceThresholds Thresholds        `json:"acceptanceThresholds"`
	Status               CaseStatus        `json:"status"`
	Version              int64             `json:"version"`
	Formulas             []FormulaRevision `json:"formulas"`
	Patches              []TrialPatch      `json:"patches"`
	Deviations           []Deviation       `json:"deviations"`
	Approval             *ApprovalRecord   `json:"approval,omitempty"`
	CreatedAt            time.Time         `json:"createdAt"`
	UpdatedAt            time.Time         `json:"updatedAt"`
}

func (c *RestorationCase) EnsureMutable() error {
	if c.Status == StatusApproved || c.Approval != nil {
		return ErrFrozen
	}
	return nil
}

func (c *RestorationCase) Formula(id string) (*FormulaRevision, error) {
	for i := range c.Formulas {
		if c.Formulas[i].FormulaID == id {
			return &c.Formulas[i], nil
		}
	}
	return nil, ErrNotFound
}

func (c *RestorationCase) Patch(id string) (*TrialPatch, error) {
	for i := range c.Patches {
		if c.Patches[i].PatchID == id {
			return &c.Patches[i], nil
		}
	}
	return nil, ErrNotFound
}

func (c *RestorationCase) Deviation(id string) (*Deviation, error) {
	for i := range c.Deviations {
		if c.Deviations[i].DeviationID == id {
			return &c.Deviations[i], nil
		}
	}
	return nil, ErrNotFound
}

func (c *RestorationCase) NextFormulaRevision() int {
	maximum := 0
	for _, formula := range c.Formulas {
		if formula.RevisionNumber > maximum {
			maximum = formula.RevisionNumber
		}
	}
	return maximum + 1
}

func (c *RestorationCase) HasPatchCode(code string) bool {
	for _, patch := range c.Patches {
		if patch.PatchCode == code {
			return true
		}
	}
	return false
}

func (c *RestorationCase) Touch(now time.Time) {
	c.Version++
	c.UpdatedAt = now.UTC()
}
