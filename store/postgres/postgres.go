package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/collapsinghierarchy/noisybuffer/model"
	"github.com/collapsinghierarchy/noisybuffer/store"
)

type pgStore struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) store.Store {
	return &pgStore{db: db}
}

func (p *pgStore) InsertSubmission(ctx context.Context, s *model.Submission) error {
	_, err := p.db.Exec(ctx,
		`INSERT INTO submissions (id, app_id, kid, ts, blob)
         VALUES ($1,$2,$3,$4,$5)`,
		s.ID, s.AppID, s.Kid, s.TS, s.Blob)
	return err
}

func (p *pgStore) StreamSubmissions(ctx context.Context, appID uuid.UUID, fn func(*model.Submission) error) error {
	rows, err := p.db.Query(ctx,
		`SELECT id, app_id, kid, ts, blob
         FROM submissions
         WHERE app_id=$1
         ORDER BY ts ASC`, appID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var s model.Submission
		if err := rows.Scan(&s.ID, &s.AppID, &s.Kid, &s.TS, &s.Blob); err != nil {
			return err
		}
		if err := fn(&s); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (p *pgStore) AppExists(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	err := p.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM apps WHERE id=$1)`, id).Scan(&exists)
	return exists, err
}
