package model

import (
	"time"

	"github.com/google/uuid"
)

type Submission struct {
	ID    uuid.UUID
	AppID uuid.UUID
	Kid   uint8
	TS    time.Time
	Blob  []byte
}

type App struct {
	ID         uuid.UUID
	Name       string
	CurrentKid uint8
	PubKey     []byte
}
