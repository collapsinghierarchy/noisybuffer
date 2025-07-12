package store

import (
	"context"

	"github.com/collapsinghierarchy/noisybuffer/model"
	"github.com/google/uuid"
)

type Store interface {
	// submissions
	InsertSubmission(ctx context.Context, s *model.Submission) error
	StreamSubmissions(ctx context.Context, appID uuid.UUID, fn func(*model.Submission) error) error

	// apps / keys
	AppExists(ctx context.Context, id uuid.UUID) (bool, error)
	RegisterKey(ctx context.Context, appID uuid.UUID, kid uint8, pub []byte) error
	GetKey(ctx context.Context, appID uuid.UUID) (kid uint8, pub []byte, err error)
}
