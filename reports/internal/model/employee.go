package model

import (
	"time"
	"uuid"
)

type Employee struct {
	ID        uuid.UUID
	FIO       FIO
	BirthDate time.Time
	IsFired   bool
}
