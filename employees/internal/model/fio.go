package model

import (
	"regexp"
	"strings"

	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/errors"
)

var fioRegexp = regexp.MustCompile(
	`^[А-ЯЁ][а-яё]+(?:-[А-ЯЁ][а-яё]+)*(?:\s+[А-ЯЁ][а-яё]+(?:-[А-ЯЁ][а-яё]+)*){1,2}$`,
)

type FIO struct{ value string }

func NewFIO(value string) (FIO, error) {
	value = strings.TrimSpace(value)

	if !fioRegexp.MatchString(value) {
		return FIO{}, errors.ErrInvalidFIO
	}

	return FIO{value}, nil
}

func (f FIO) Value() string {
	return f.value
}
