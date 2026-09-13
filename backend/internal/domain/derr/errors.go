// Package derr はドメイン横断のエラー型を定義する。
package derr

import (
	"errors"
	"fmt"
)

// ErrNotFound は対象リソースが存在しない場合のセンチネルエラー。
var ErrNotFound = errors.New("resource not found")

// ErrConflict は状態遷移が許可されない（例: 処理済み予定案の再承認）場合のエラー。
var ErrConflict = errors.New("conflict")

// ValidationError は入力バリデーション失敗を表す。
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error on %q: %s", e.Field, e.Message)
	}
	return "validation error: " + e.Message
}

// NewValidationError はバリデーションエラーを生成する。
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

// IsValidation は err がバリデーションエラーかどうかを返す。
func IsValidation(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}
