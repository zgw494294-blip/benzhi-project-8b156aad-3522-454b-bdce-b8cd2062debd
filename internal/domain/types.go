package domain

import "time"

type CaseStatus string

const (
	StatusDraft       CaseStatus = "draft"
	StatusTesting     CaseStatus = "testing"
	StatusRemediation CaseStatus = "remediation"
	StatusPending     CaseStatus = "pending_review"
	StatusApproved    CaseStatus = "approved"
)

type CuringStage string

const (
	StageInitial CuringStage = "initial"
	StageStable  CuringStage = "stable"
	StageFinal   CuringStage = "final"
)

var stages = []CuringStage{StageInitial, StageStable, StageFinal}

func StageIndex(stage CuringStage) int {
	for i, value := range stages {
		if value == stage {
			return i
		}
	}
	return -1
}

type Decision string

const (
	DecisionApprove Decision = "approve"
	DecisionReturn  Decision = "return"
)

type DeviationStatus string

const (
	DeviationOpen       DeviationStatus = "open"
	DeviationRemediated DeviationStatus = "remediated"
	DeviationClosed     DeviationStatus = "closed"
)

type Thresholds struct {
	MaxColorDifference  float64 `json:"maxColorDifference"`
	MaxWaterAbsorption  float64 `json:"maxWaterAbsorption"`
	MinAdhesionStrength float64 `json:"minAdhesionStrength"`
}

type Ingredient struct {
	Name       string  `json:"name"`
	Percentage float64 `json:"percentage"`
	BatchNote  string  `json:"batchNote,omitempty"`
}

type FormulaRevision struct {
	FormulaID           string       `json:"formulaID"`
	CaseID              string       `json:"caseID"`
	RevisionNumber      int          `json:"revisionNumber"`
	Ingredients         []Ingredient `json:"ingredients"`
	ApplicationMethod   string       `json:"applicationMethod"`
	SubstrateConditions string       `json:"substrateConditions"`
	ChangeReason        string       `json:"changeReason"`
	CreatedBy           string       `json:"createdBy"`
	CreatedAt           time.Time    `json:"createdAt"`
}

type Observation struct {
	ObservationID    string      `json:"observationID"`
	PatchID          string      `json:"patchID"`
	Stage            CuringStage `json:"stage"`
	ColorDifference  float64     `json:"colorDifference"`
	WaterAbsorption  float64     `json:"waterAbsorption"`
	AdhesionStrength float64     `json:"adhesionStrength"`
	SurfaceDefects   []string    `json:"surfaceDefects"`
	ObservedAt       time.Time   `json:"observedAt"`
	EvidenceSummary  string      `json:"evidenceSummary"`
	RecordedBy       string      `json:"recordedBy"`
}

type TrendWarning struct {
	Code                 string  `json:"code"`
	Metric               string  `json:"metric"`
	Change               float64 `json:"change"`
	ThresholdMargin      float64 `json:"thresholdMargin"`
	ConsecutiveWorsening bool    `json:"consecutiveWorsening"`
}

type MetricTrend struct {
	Metric               string  `json:"metric"`
	Change               float64 `json:"change"`
	ThresholdMargin      float64 `json:"thresholdMargin"`
	Worsening            bool    `json:"worsening"`
	ConsecutiveWorsening bool    `json:"consecutiveWorsening"`
}

type StageTrend struct {
	Stage      CuringStage    `json:"stage"`
	ObservedAt time.Time      `json:"observedAt"`
	Metrics    []MetricTrend  `json:"metrics"`
	Warnings   []TrendWarning `json:"warnings"`
}

type EvaluationResult struct {
	Conclusion     string    `json:"conclusion"`
	EvaluatedAt    time.Time `json:"evaluatedAt"`
	DeviationCount int       `json:"deviationCount"`
}

type TrialPatch struct {
	PatchID          string            `json:"patchID"`
	CaseID           string            `json:"caseID"`
	FormulaID        string            `json:"formulaID"`
	PatchCode        string            `json:"patchCode"`
	ParentPatchID    string            `json:"parentPatchID,omitempty"`
	RootPatchID      string            `json:"rootPatchID"`
	RetestRound      int               `json:"retestRound"`
	CuringStage      CuringStage       `json:"curingStage,omitempty"`
	Observations     []Observation     `json:"observations"`
	Trends           []StageTrend      `json:"trends"`
	EvaluationResult *EvaluationResult `json:"evaluationResult,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	CompletedAt      *time.Time        `json:"completedAt,omitempty"`
}

type Deviation struct {
	DeviationID          string          `json:"deviationID"`
	CaseID               string          `json:"caseID"`
	PatchID              string          `json:"patchID"`
	Metric               string          `json:"metric"`
	MeasuredValue        string          `json:"measuredValue"`
	Threshold            string          `json:"threshold"`
	Severity             string          `json:"severity"`
	Cause                string          `json:"cause,omitempty"`
	Disposition          string          `json:"disposition,omitempty"`
	Status               DeviationStatus `json:"status"`
	ReplacementFormulaID string          `json:"replacementFormulaID,omitempty"`
	RetestPatchID        string          `json:"retestPatchID,omitempty"`
	ClosedAt             *time.Time      `json:"closedAt,omitempty"`
}

type ApprovalRecord struct {
	ApprovalID        string    `json:"approvalID"`
	CaseID            string    `json:"caseID"`
	Decision          Decision  `json:"decision"`
	ReviewComment     string    `json:"reviewComment"`
	ApprovedFormulaID string    `json:"approvedFormulaID,omitempty"`
	FrozenCaseVersion int64     `json:"frozenCaseVersion"`
	SnapshotDigest    string    `json:"snapshotDigest"`
	SnapshotJSON      string    `json:"snapshotJSON,omitempty"`
	DecidedBy         string    `json:"decidedBy"`
	DecidedAt         time.Time `json:"decidedAt"`
}

type AuditEvent struct {
	EventID    string         `json:"eventID"`
	CaseID     string         `json:"caseID"`
	EventType  string         `json:"eventType"`
	Summary    string         `json:"summary"`
	Actor      string         `json:"actor"`
	Data       map[string]any `json:"data,omitempty"`
	OccurredAt time.Time      `json:"occurredAt"`
}
