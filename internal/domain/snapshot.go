package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	VerificationVerified       = "verified"
	VerificationDigestMismatch = "digest_mismatch"
	VerificationUnparseable    = "snapshot_unparseable"
)

type FrozenSnapshot struct {
	CaseID          string          `json:"caseID"`
	Version         int64           `json:"version"`
	Thresholds      Thresholds      `json:"thresholds"`
	ApprovedFormula FormulaRevision `json:"approvedFormula"`
	Patches         []TrialPatch    `json:"patches"`
	Deviations      []Deviation     `json:"deviations"`
}

type ApprovalEvidence struct {
	Approval           ApprovalRecord `json:"approval"`
	VerificationStatus string         `json:"verificationStatus"`
	Evidence           FrozenSnapshot `json:"evidence"`
}

func BuildSnapshot(c *RestorationCase, approvedFormulaID string) (string, string, error) {
	if err := PopulateDerivedEvidence(c); err != nil {
		return "", "", err
	}
	formula, err := c.Formula(approvedFormulaID)
	if err != nil {
		return "", "", Invalid("approvedFormulaID", "获批配方不存在")
	}
	if !FormulaHasPassingPatch(c, approvedFormulaID) {
		return "", "", Invalid("approvedFormulaID", "获批配方必须具有关联的合格终期试验块")
	}
	snapshot := FrozenSnapshot{CaseID: c.CaseID, Version: c.Version + 1, Thresholds: c.AcceptanceThresholds, ApprovedFormula: *formula, Patches: c.Patches, Deviations: c.Deviations}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return "", "", err
	}
	digest := sha256.Sum256(data)
	return string(data), hex.EncodeToString(digest[:]), nil
}

func VerifyApprovalSnapshot(record *ApprovalRecord) (*ApprovalEvidence, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(record.SnapshotJSON))
	decoder.DisallowUnknownFields()
	var snapshot FrozenSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, integrityFailure(VerificationUnparseable, "冻结快照不可解析")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, integrityFailure(VerificationUnparseable, "冻结快照包含额外内容")
	}
	if snapshot.CaseID != record.CaseID || snapshot.Version != record.FrozenCaseVersion || snapshot.ApprovedFormula.FormulaID != record.ApprovedFormulaID {
		return nil, integrityFailure(VerificationUnparseable, "冻结快照关键标识不一致")
	}
	if err := ValidateThresholds(snapshot.Thresholds); err != nil {
		return nil, integrityFailure(VerificationUnparseable, "冻结阈值不可解析")
	}
	if err := ValidateFormula(snapshot.ApprovedFormula); err != nil {
		return nil, integrityFailure(VerificationUnparseable, "冻结配方不可解析")
	}
	digest := sha256.Sum256([]byte(record.SnapshotJSON))
	actual := hex.EncodeToString(digest[:])
	if actual != record.SnapshotDigest {
		return nil, integrityFailure(VerificationDigestMismatch, "冻结快照摘要与批准记录不匹配")
	}
	return &ApprovalEvidence{Approval: *record, VerificationStatus: VerificationVerified, Evidence: snapshot}, nil
}

func integrityFailure(status, message string) error {
	return errors.Join(ErrIntegrity, &IntegrityError{VerificationStatus: status, Message: fmt.Sprintf("批准证据完整性错误：%s", message)})
}

func FormulaHasPassingPatch(c *RestorationCase, formulaID string) bool {
	for _, patch := range c.Patches {
		if patch.FormulaID != formulaID || patch.EvaluationResult == nil {
			continue
		}
		if patch.EvaluationResult.Conclusion == "passed" {
			return true
		}
	}
	return false
}
