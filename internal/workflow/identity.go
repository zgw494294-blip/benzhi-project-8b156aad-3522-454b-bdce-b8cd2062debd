package workflow

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

func newID(prefix string) string {
	data := make([]byte, 12)
	if _, err := rand.Read(data); err != nil {
		panic(fmt.Sprintf("生成标识失败: %v", err))
	}
	return prefix + "_" + hex.EncodeToString(data)
}

func requestHash(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validateMeta(meta CommandMeta) error {
	if meta.ExpectedVersion < 1 {
		return fmt.Errorf("%w: expectedVersion 必须大于零", domainValidation())
	}
	if len(strings.TrimSpace(meta.IdempotencyKey)) < 8 {
		return fmt.Errorf("%w: idempotencyKey 至少 8 个字符", domainValidation())
	}
	if strings.TrimSpace(meta.Actor) == "" {
		return fmt.Errorf("%w: actor 不能为空", domainValidation())
	}
	return nil
}

// 避免调用方依赖具体校验错误文本，同时保留 errors.Is 语义。
func domainValidation() error { return errValidation }
