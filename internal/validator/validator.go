package validator

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
)

var codeRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Validator struct {
	Errors map[string]string
}

func New() *Validator {
	return &Validator{Errors: make(map[string]string)}
}

func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

func (v *Validator) RequiredString(value, key string) {
	if strings.TrimSpace(value) == "" {
		v.AddError(key, fmt.Sprintf("%s is required", key))
	}
}

func (v *Validator) Email(email, key string) {
	v.RequiredString(email, key)
	if _, err := mail.ParseAddress(email); err != nil {
		v.AddError(key, "invalid email address format")
	}
}

func (v *Validator) TenantCode(code, key string) {
	v.RequiredString(code, key)
	if len(code) < 3 || len(code) > 64 {
		v.AddError(key, "tenant code must be between 3 and 64 characters")
		return
	}
	if !codeRegex.MatchString(code) {
		v.AddError(key, "tenant code must be lowercase alphanumeric with hyphens (e.g. 'acme-corp')")
	}
}

func (v *Validator) MinLength(value, key string, min int) {
	if len(strings.TrimSpace(value)) < min {
		v.AddError(key, fmt.Sprintf("%s must be at least %d characters", key, min))
	}
}

func (v *Validator) In(value, key string, allowed ...string) {
	for _, a := range allowed {
		if value == a {
			return
		}
	}
	v.AddError(key, fmt.Sprintf("%s must be one of: %s", key, strings.Join(allowed, ", ")))
}

func (v *Validator) ToAppError() *domain.AppError {
	if v.Valid() {
		return nil
	}
	details := make(map[string]interface{})
	for k, val := range v.Errors {
		details[k] = val
	}
	return &domain.AppError{
		Status:  422,
		Code:    "VALIDATION_ERROR",
		Message: "request validation failed",
		Details: details,
	}
}
