package service

import (
	"context"
	"errors"
	"time"

	"github.com/collapsinghierarchy/noisybuffer/model"
	"github.com/collapsinghierarchy/noisybuffer/store"
	"github.com/google/uuid"
)

var (
	ErrAppNotFound = errors.New("app not found")
)

type Service struct {
	store   store.Store // dependency-injected DAL interface
	maxBlob int64       // configurable size guard
}

func New(st store.Store, maxBlob int64) *Service {
	return &Service{store: st, maxBlob: maxBlob}
}

func (s *Service) Submit(ctx context.Context, appID uuid.UUID, kid uint8, blob []byte) error {
	if int64(len(blob)) > s.maxBlob {
		return errors.New("blob too large")
	}
	exists, err := s.store.AppExists(ctx, appID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrAppNotFound
	}
	sub := &model.Submission{
		ID:    uuid.New(),
		AppID: appID,
		Kid:   kid,
		TS:    time.Now().UTC(),
		Blob:  blob,
	}
	return s.store.InsertSubmission(ctx, sub)
}

func (s *Service) Export(ctx context.Context, appID uuid.UUID, fn func(*model.Submission) error) error {
	return s.store.StreamSubmissions(ctx, appID, fn)
}
