package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"stone-restoration-trial/internal/domain"
)

type envelope struct {
	Data any `json:"data,omitempty"`
	Meta any `json:"meta,omitempty"`
}
type errorEnvelope struct {
	Error apiError `json:"error"`
}
type apiError struct {
	Code               string                  `json:"code"`
	Message            string                  `json:"message"`
	Field              string                  `json:"field,omitempty"`
	Issues             []domain.ReadinessIssue `json:"issues,omitempty"`
	VerificationStatus string                  `json:"verificationStatus,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func decodeJSON(r *http.Request, target any) error {
	if r.Header.Get("Content-Type") != "application/json" && r.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		return domain.Invalid("contentType", "Content-Type 必须为 application/json")
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, (1<<20)+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return domain.Invalid("body", "JSON 请求体无效: "+err.Error())
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return domain.Invalid("body", "请求体只能包含一个 JSON 对象")
	}
	return nil
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "服务内部错误"
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		status = 499
		code = "request_canceled"
		message = "请求已取消，未提交任何变更"
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		code = "not_found"
		message = "请求的记录不存在"
	case errors.Is(err, domain.ErrConflict):
		status = http.StatusConflict
		code = "version_conflict"
		message = "任务版本已变化，请刷新后重试"
	case errors.Is(err, domain.ErrIdempotencyReuse):
		status = http.StatusConflict
		code = "idempotency_reuse"
		message = "幂等键已用于不同请求"
	case errors.Is(err, domain.ErrDuplicate):
		status = http.StatusConflict
		code = "duplicate"
		message = "相同标识的记录已存在"
	case errors.Is(err, domain.ErrFrozen):
		status = http.StatusConflict
		code = "case_frozen"
		message = "任务已批准冻结，不能继续变更"
	case errors.Is(err, domain.ErrInvalidState):
		status = http.StatusUnprocessableEntity
		code = "invalid_state"
		message = "当前任务状态不允许此操作"
	case errors.Is(err, domain.ErrIntegrity):
		status = http.StatusConflict
		code = "approval_integrity_error"
		message = "批准证据完整性校验失败"
	case errors.Is(err, domain.ErrValidation):
		status = http.StatusBadRequest
		code = "validation_error"
		message = err.Error()
	}
	var rule *domain.RuleError
	field := ""
	issues := []domain.ReadinessIssue(nil)
	verificationStatus := ""
	if errors.As(err, &rule) {
		message = rule.Message
		field = rule.Field
	}
	var readiness *domain.ReadinessError
	if errors.As(err, &readiness) {
		status = http.StatusUnprocessableEntity
		code = "review_blocked"
		message = readiness.Error()
		issues = readiness.Issues
	}
	var integrity *domain.IntegrityError
	if errors.As(err, &integrity) {
		message = integrity.Message
		verificationStatus = integrity.VerificationStatus
	}
	writeJSON(w, status, errorEnvelope{Error: apiError{Code: code, Message: message, Field: field, Issues: issues, VerificationStatus: verificationStatus}})
}

func replayMeta(replayed bool) map[string]any { return map[string]any{"idempotentReplay": replayed} }
