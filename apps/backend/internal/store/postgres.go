package store

import (
	"backend/internal/models"
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var ErrNotFound = errors.New("link not found")

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", dsn)

	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func (s *PostgresStore) DB() *sql.DB {
	return s.db
}

func (s *PostgresStore) CreateLink(ctx context.Context, slug, targetURL string) (*models.Link, error) {
	var link models.Link

	query := `
		insert into links (slug, target_url, created_at)
		values ($1, $2, now())
		returning id, slug, target_url, created_at`

	err := s.db.QueryRowContext(ctx, query, slug, targetURL).
		Scan(&link.ID, &link.Slug, &link.TargetURL, &link.CreatedAt)

	if err != nil {
		return nil, err
	}

	return &link, nil
}

func (s *PostgresStore) GetLinkBySlug(ctx context.Context, slug string) (*models.Link, error) {
	var link models.Link

	query := `select id, slug, target_url, created_at from links where slug = $1`

	err := s.db.QueryRowContext(ctx, query, slug).
		Scan(&link.ID, &link.Slug, &link.TargetURL, &link.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &link, nil
}

func (s *PostgresStore) CountClicks(ctx context.Context, slug string) (int64, error) {
	var count int64

	query := `
		select count(*) from click_events ce
		join links l on l.id = ce.link_id
		where l.slug = $1`

	if err := s.db.QueryRowContext(ctx, query, slug).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func (s *PostgresStore) SlugExists(ctx context.Context, slug string) (bool, error) {
	var exists bool

	query := `select exists(select 1 from links where slug = $1)`

	if err := s.db.QueryRowContext(ctx, query, slug).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}
