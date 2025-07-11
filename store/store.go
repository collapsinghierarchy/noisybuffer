package store

import (
	"context"

	"github.com/collapsinghierarchy/noisybuffer/model"
	"github.com/google/uuid"
)

type Store interface {
	InsertSubmission(ctx context.Context, s *model.Submission) error
	StreamSubmissions(ctx context.Context, projectID uuid.UUID, fn func(*model.Submission) error) error
	AppExists(ctx context.Context, id uuid.UUID) (bool, error)
}
