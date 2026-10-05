package rss

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const siteSchema = `
CREATE TABLE IF NOT EXISTS sites (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    position INTEGER NOT NULL CHECK (position > 0)
)`

// objectStore は sites.db の読み書き先。実装は Cloudflare R2。
type objectStore interface {
	Get(ctx context.Context, bucket, key string) ([]byte, error)
	Put(ctx context.Context, bucket, key string, data []byte) error
}

var errNotFound = errors.New("オブジェクトがありません")

// LoadSites は R2 上の SQLite から登録サイトを読む。
// オブジェクトが無いときは既定のサイトで sites.db を作り、R2 へ書く。
func LoadSites(ctx context.Context, store objectStore, bucket, key string) ([]Site, error) {
	var sites []Site
	err := withSiteDB(ctx, store, bucket, key, false, func(db *siteDB) error {
		var err error
		sites, err = db.list(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return sites, nil
}

// AddSite はサイトを登録するか、同じ ID の名前と URL を更新し、SQLite を R2 へ書く。
func AddSite(ctx context.Context, store objectStore, bucket, key string, site Site) error {
	site.Name = strings.TrimSpace(site.Name)
	site.URL = strings.TrimSpace(site.URL)
	if err := Validate([]Site{site}); err != nil {
		return err
	}
	return withSiteDB(ctx, store, bucket, key, true, func(db *siteDB) error {
		return db.upsert(ctx, site)
	})
}

// RemoveSite は ID のサイトを消し、SQLite を R2 へ書く。
func RemoveSite(ctx context.Context, store objectStore, bucket, key string, id string) error {
	if !idPattern.MatchString(id) || len(id) > 64 {
		return fmt.Errorf("サイトIDが不正です: %q", id)
	}
	return withSiteDB(ctx, store, bucket, key, true, func(db *siteDB) error {
		return db.remove(ctx, id)
	})
}

// withSiteDB は R2 から SQLite を一時ファイルへ下ろして fn を実行する。
// オブジェクトが無いときは既定サイトで新規作成する。
// write が真で、かつファイルが変わったときだけ R2 へ戻す。
func withSiteDB(ctx context.Context, store objectStore, bucket, key string, write bool, fn func(*siteDB) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateLocation(bucket, key); err != nil {
		return err
	}
	original, err := store.Get(ctx, bucket, key)
	missing := errors.Is(err, errNotFound)
	if err != nil && !missing {
		return err
	}
	if missing {
		original = nil
		write = true
	}
	if len(original) > 0 && !bytes.HasPrefix(original, []byte("SQLite format 3\x00")) {
		return fmt.Errorf("R2上の %s はSQLiteデータベースではありません", key)
	}

	dir, err := os.MkdirTemp("", "rss-sites-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "sites.db")
	if len(original) > 0 {
		if err := os.WriteFile(path, original, 0o600); err != nil {
			return err
		}
	}

	db, err := openSiteDB(path, write)
	if err != nil {
		return err
	}
	defer db.close()

	if missing {
		if err := db.seed(ctx, defaultSites); err != nil {
			return err
		}
	}
	if err := fn(db); err != nil {
		return err
	}
	if err := db.close(); err != nil {
		return err
	}
	if !write {
		return nil
	}
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		if _, err := os.Stat(path + suffix); err == nil {
			return fmt.Errorf("データベースの書き出しが完了していません")
		}
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if bytes.Equal(updated, original) {
		return nil
	}
	return store.Put(ctx, bucket, key, updated)
}

type siteDB struct {
	db *sql.DB
}

func openSiteDB(path string, write bool) (*siteDB, error) {
	dsn, err := sqliteDSN(path, write)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("サイト一覧を開けません: %w", err)
	}
	site := &siteDB{db: db}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		site.close()
		return nil, fmt.Errorf("サイト一覧を開けません: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		site.close()
		return nil, err
	}
	if write {
		var mode string
		if err := db.QueryRow(`PRAGMA journal_mode = DELETE`).Scan(&mode); err != nil {
			site.close()
			return nil, err
		}
		if mode != "delete" {
			site.close()
			return nil, fmt.Errorf("journal_mode を delete にできません: %s", mode)
		}
		if _, err := db.Exec(siteSchema); err != nil {
			site.close()
			return nil, fmt.Errorf("サイト一覧の表を作れません: %w", err)
		}
	}
	var check string
	if err := db.QueryRow(`PRAGMA quick_check`).Scan(&check); err != nil {
		site.close()
		return nil, fmt.Errorf("サイト一覧を読めません: %w", err)
	}
	if check != "ok" {
		site.close()
		return nil, fmt.Errorf("サイト一覧が壊れています: %s", check)
	}
	return site, nil
}

func sqliteDSN(path string, write bool) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("データベースのパスが絶対パスではありません")
	}
	mode := "ro"
	if write {
		mode = "rwc"
	}
	return "file:" + filepath.ToSlash(path) + "?mode=" + mode, nil
}

func (s *siteDB) close() error {
	if s == nil || s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *siteDB) list(ctx context.Context) ([]Site, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, url FROM sites ORDER BY position, id`)
	if err != nil {
		return nil, fmt.Errorf("サイト一覧を読めません: %w", err)
	}
	defer rows.Close()
	var sites []Site
	for rows.Next() {
		var site Site
		if err := rows.Scan(&site.ID, &site.Name, &site.URL); err != nil {
			return nil, err
		}
		sites = append(sites, site)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("サイト一覧を読めません: %w", err)
	}
	return sites, nil
}

func (s *siteDB) seed(ctx context.Context, sites []Site) error {
	if err := Validate(sites); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, site := range sites {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO sites (id, name, url, position) VALUES (?, ?, ?, ?)`,
			site.ID, site.Name, site.URL, i+1)
		if err != nil {
			return fmt.Errorf("初期サイトを書けません: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *siteDB) upsert(ctx context.Context, site Site) error {
	site.Name = strings.TrimSpace(site.Name)
	site.URL = strings.TrimSpace(site.URL)
	if err := Validate([]Site{site}); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sites WHERE id = ?`, site.ID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE sites SET name = ?, url = ? WHERE id = ?`, site.Name, site.URL, site.ID); err != nil {
			return err
		}
		return tx.Commit()
	}
	var pos int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) FROM sites`).Scan(&pos); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO sites (id, name, url, position) VALUES (?, ?, ?, ?)`,
		site.ID, site.Name, site.URL, pos+1); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *siteDB) remove(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sites WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("サイトが登録されていません: %s", id)
	}
	return nil
}
