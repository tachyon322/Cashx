package platform

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// ValidateStruct validates v with go-playground/validator and returns a
// user-facing message for the first failing rule.
func ValidateStruct(v any) error {
	if err := validate.Struct(v); err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			return fmt.Errorf("invalid validation input: %w", err)
		}
		for _, fe := range err.(validator.ValidationErrors) {
			return fmt.Errorf("validation failed on field %q (rule %s)", fe.Field(), fe.Tag())
		}
	}
	return nil
}
