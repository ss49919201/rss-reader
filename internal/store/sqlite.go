package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"time"

	"github.com/ss49919201/rss-reader/internal/site"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

const maxNotifiedLinks = 500

// SQLite はファイル上の SQLite データベース。
type SQLite struct {
	db   *sql.DB
	path string
}

// OpenFile は path の DB を開き、スキーマを適用する。
func OpenFile(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &SQLite{db: db, path: path}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// CreateEmptyFile は新しい空の DB ファイルを作る。
func CreateEmptyFile(path string) (*SQLite, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return OpenFile(path)
}

func (s *SQLite) migrate() error {
	_, err := s.db.Exec(schemaSQL)
	return err
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

// ExportBytes は DB ファイルの内容を読み出す。
func (s *SQLite) ExportBytes() ([]byte, error) {
	if s.path == "" {
		return nil, fmt.Errorf("DB パスが未設定です")
	}
	return os.ReadFile(s.path)
}

func (s *SQLite) ListSites(ctx context.Context) ([]site.Site, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, url FROM sites ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sites []site.Site
	for rows.Next() {
		var row site.Site
		if err := rows.Scan(&row.ID, &row.Name, &row.URL); err != nil {
			return nil, err
		}
		sites = append(sites, row)
	}
	return sites, rows.Err()
}

func (s *SQLite) AddSite(ctx context.Context, st site.Site) error {
	if err := site.Validate([]site.Site{st}); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO sites (id, name, url) VALUES (?, ?, ?)`, st.ID, st.Name, st.URL)
	if err != nil {
		return fmt.Errorf("サイトを追加できません: %w", err)
	}
	return nil
}

func (s *SQLite) RemoveSite(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sites WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("サイトが見つかりません: %s", id)
	}
	return nil
}

func (s *SQLite) LastRun(ctx context.Context) (time.Time, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT last_run FROM run_state WHERE id = 1`).Scan(&raw)
	if err != nil {
		return time.Time{}, err
	}
	if raw == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("last_run を解釈できません: %w", err)
	}
	return t, nil
}

func (s *SQLite) SetLastRun(ctx context.Context, t time.Time) error {
	raw := ""
	if !t.IsZero() {
		raw = t.UTC().Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE run_state SET last_run = ? WHERE id = 1`, raw)
	return err
}

func (s *SQLite) HasNotifiedLink(ctx context.Context, link string) (bool, error) {
	if link == "" {
		return false, nil
	}
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM notified_links WHERE link = ?`, link).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *SQLite) RememberLink(ctx context.Context, link string, at time.Time) error {
	if link == "" {
		return nil
	}
	atRaw := at.UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO notified_links (link, notified_at) VALUES (?, ?)`, link, atRaw)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
DELETE FROM notified_links WHERE link NOT IN (
  SELECT link FROM notified_links ORDER BY notified_at DESC LIMIT ?
)`, maxNotifiedLinks)
	return err
}

