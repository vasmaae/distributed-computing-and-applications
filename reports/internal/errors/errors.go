package errors

import (
	"errors"
)

var (
	ErrInvalidFIO = errors.New("fio is invalid")

	ErrEmployeesClient = errors.New("employees client error")
	ErrParseEmployee   = errors.New("error parsing employee")
)
