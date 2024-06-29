package validator

import (
	"regexp"

	"github.com/go-playground/validator/v10"
)

func ValidateMobile(f1 validator.FieldLevel) bool {
	mobile := f1.Field().String()
	ok, _ := regexp.MatchString(`^1([38][0-9]|14[579]|5[^4]|16[6]|7[1-35-8]|9[189])\d{8}$`, mobile)
	return ok
}

func ValidateEmail(f1 validator.FieldLevel) bool {
	email := f1.Field().String()
	ok, _ := regexp.MatchString(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, email)
	return ok
}
