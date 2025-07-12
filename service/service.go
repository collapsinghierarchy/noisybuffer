package service

import (
	"context"
	"database/sql"
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
	Store   store.Store // dependency-injected DAL interface
	maxBlob int64       // configurable size guard
	kemPub  []byte
	kid     uint8
}

func New(st store.Store, maxBlob int64) *Service {
	return &Service{Store: st, maxBlob: maxBlob}
}

var (
	ErrKeyNotFound = errors.New("public key not registered")
	ErrKeyExists   = errors.New("public key already registered")
)

func (s *Service) RegisterKey(ctx context.Context, appID uuid.UUID, kid uint8, pub []byte) error {
	return s.Store.RegisterKey(ctx, appID, kid, pub)
}

func (s *Service) GetKey(ctx context.Context, appID uuid.UUID) (uint8, []byte, error) {
	kid, pub, err := s.Store.GetKey(ctx, appID)
	if errors.Is(err, sql.ErrNoRows) { // adapter returns sql.ErrNoRows
		return 0, nil, ErrKeyNotFound
	}
	return kid, pub, err
}

func (s *Service) Push(ctx context.Context, appID uuid.UUID, kid uint8, blob []byte) error {
	if int64(len(blob)) > s.maxBlob {
		return errors.New("blob too large")
	}
	exists, err := s.Store.AppExists(ctx, appID)
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
	return s.Store.InsertSubmission(ctx, sub)
}

func (s *Service) Pull(ctx context.Context, appID uuid.UUID, fn func(*model.Submission) error) error {
	return s.Store.StreamSubmissions(ctx, appID, fn)
}
