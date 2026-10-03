package errors

import (
	"errors"
)

var (
	ErrEmployeeNotFound = errors.New("employee not found")
	ErrInvalidFIO       = errors.New("fio is invalid")
	ErrInvalidBirthDate = errors.New("invalid birthdate")
)
