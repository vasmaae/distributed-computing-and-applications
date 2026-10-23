package errors

import (
	"errors"
)

var (
	ErrFileNotFound     = errors.New("file not found")
	ErrEmployeeNotFound = errors.New("employee not found")
	ErrInvalidFIO       = errors.New("fio is invalid")
	ErrInvalidBirthDate = errors.New("invalid birthdate")
	ErrInvalidSort      = errors.New("invalid sort options")

	ErrReportsClient = errors.New("reports client error")
)
