package domain

import "errors"

var (
	ErrNotFound         = errors.New("记录不存在")
	ErrConflict         = errors.New("版本冲突")
	ErrValidation       = errors.New("输入校验失败")
	ErrInvalidState     = errors.New("当前状态不允许此操作")
	ErrFrozen           = errors.New("已批准任务不可变更")
	ErrIdempotencyReuse = errors.New("幂等键已用于不同请求")
	ErrDuplicate        = errors.New("记录已存在")
	ErrIntegrity        = errors.New("批准快照完整性校验失败")
)

type IntegrityError struct {
	VerificationStatus string `json:"verificationStatus"`
	Message            string `json:"message"`
}

func (e *IntegrityError) Error() string { return e.Message }

type RuleError struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

func (e *RuleError) Error() string { return e.Message }

func Invalid(field, message string) error {
	return errors.Join(ErrValidation, &RuleError{Field: field, Message: message})
}
