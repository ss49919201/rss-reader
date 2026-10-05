// Package sitedb は登録サイトを Cloudflare R2 上の SQLite ファイルに保存する。
//
// R2 のオブジェクトが正本で、ローカルには処理中の一時ファイルしか置かない。
package sitedb

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	_ "modernc.org/sqlite"

	"github.com/ss49919201/rss-reader/internal/rss"
)

// DefaultObjectKey は R2_OBJECT_KEY を指定しないときのオブジェクトキー。
const DefaultObjectKey = "sites.db"

const maxUpdateAttempts = 5

// defaultSites は R2 にまだデータベースがないときに最初に登録するサイト。
var defaultSites = []rss.Site{
	{ID: "go-blog", Name: "Go Blog", URL: "https://go.dev/blog/feed.atom"},
	{ID: "zenn", Name: "Zenn", URL: "https://zenn.dev/feed"},
	{ID: "github-blog", Name: "GitHub Blog", URL: "https://github.blog/feed/"},
	{ID: "hatena-hotentry", Name: "はてなブックマーク 人気エントリー", URL: "https://b.hatena.ne.jp/hotentry.rss"},
}

const schema = `CREATE TABLE IF NOT EXISTS sites (
	id   TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	url  TEXT NOT NULL
)`

// Config は R2 への接続設定。
type Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	Key             string
	// Endpoint を空にすると https://<AccountID>.r2.cloudflarestorage.com を使う。
	Endpoint string
}

// ConfigFromEnv は R2_* 環境変数から設定を読む。
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		AccountID:       strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID")),
		AccessKeyID:     strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")),
		SecretAccessKey: strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY")),
		Bucket:          strings.TrimSpace(os.Getenv("R2_BUCKET")),
		Key:             strings.TrimSpace(os.Getenv("R2_OBJECT_KEY")),
		Endpoint:        strings.TrimSpace(os.Getenv("R2_ENDPOINT")),
	}
	if cfg.Key == "" {
		cfg.Key = DefaultObjectKey
	}
	var missing []string
	if cfg.AccountID == "" && cfg.Endpoint == "" {
		missing = append(missing, "R2_ACCOUNT_ID")
	}
	if cfg.AccessKeyID == "" {
		missing = append(missing, "R2_ACCESS_KEY_ID")
	}
	if cfg.SecretAccessKey == "" {
		missing = append(missing, "R2_SECRET_ACCESS_KEY")
	}
	if cfg.Bucket == "" {
		missing = append(missing, "R2_BUCKET")
	}
	if len(missing) > 0 {
		return cfg, fmt.Errorf("環境変数が設定されていません: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

// Store は R2 上の SQLite ファイルを読み書きする。
type Store struct {
	client *s3.Client
	bucket string
	key    string
}

// New は R2 に接続する Store を作る。
func New(cfg Config) *Store {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://" + cfg.AccountID + ".r2.cloudflarestorage.com"
	}
	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		UsePathStyle: true,
		// R2 は S3 SDK の既定の追加チェックサムを要求しない。
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	return &Store{client: client, bucket: cfg.Bucket, key: cfg.Key}
}

// Location は保存先を r2://<bucket>/<key> の形で返す。
func (s *Store) Location() string {
	return "r2://" + s.bucket + "/" + s.key
}

// Load は登録サイトを登録順に返す。R2 にデータベースがなければ既定のサイトで作ってアップロードする。
func (s *Store) Load(ctx context.Context) ([]rss.Site, error) {
	path, _, err := s.download(ctx)
	if errors.Is(err, errNotFound) {
		var sites []rss.Site
		err = s.update(ctx, func(tx *sql.Tx) error {
			var err error
			sites, err = readSites(ctx, tx)
			return err
		})
		return sites, err
	}
	if err != nil {
		return nil, err
	}
	defer os.Remove(path)

	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	return readSites(ctx, tx)
}

// Add はサイトを登録する。ID の重複や不正な URL はエラーにする。
func (s *Store) Add(ctx context.Context, site rss.Site) error {
	return s.update(ctx, func(tx *sql.Tx) error {
		sites, err := readSites(ctx, tx)
		if err != nil {
			return err
		}
		if err := rss.Validate(append(sites, site)); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO sites (id, name, url) VALUES (?, ?, ?)`, site.ID, site.Name, site.URL)
		return err
	})
}

// Remove は ID のサイトを削除する。
func (s *Store) Remove(ctx context.Context, id string) error {
	return s.update(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM sites WHERE id = ?`, id)
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
	})
}

// update は R2 からデータベースを取得して fn を適用し、取得時から変わっていなければ書き戻す。
// 別の書き込みと競合したときは取得からやり直す。
func (s *Store) update(ctx context.Context, fn func(*sql.Tx) error) error {
	for attempt := 0; attempt < maxUpdateAttempts; attempt++ {
		path, etag, err := s.download(ctx)
		created := errors.Is(err, errNotFound)
		if created {
			path, err = emptyTempFile()
		}
		if err != nil {
			return err
		}
		data, err := applyLocal(ctx, path, created, fn)
		os.Remove(path)
		if err != nil {
			return err
		}
		err = s.upload(ctx, data, etag)
		if isConflict(err) {
			continue
		}
		return err
	}
	return fmt.Errorf("%s の更新が他の書き込みと競合しました。やり直してください", s.Location())
}

func applyLocal(ctx context.Context, path string, created bool, fn func(*sql.Tx) error) ([]byte, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	err = func() error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if created {
			if err := seed(ctx, tx); err != nil {
				return err
			}
		}
		if err := fn(tx); err != nil {
			return err
		}
		return tx.Commit()
	}()
	if closeErr := db.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func seed(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, schema); err != nil {
		return err
	}
	for _, site := range defaultSites {
		if _, err := tx.ExecContext(ctx, `INSERT INTO sites (id, name, url) VALUES (?, ?, ?)`, site.ID, site.Name, site.URL); err != nil {
			return err
		}
	}
	return nil
}

func readSites(ctx context.Context, tx *sql.Tx) ([]rss.Site, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, name, url FROM sites ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sites []rss.Site
	for rows.Next() {
		var site rss.Site
		if err := rows.Scan(&site.ID, &site.Name, &site.URL); err != nil {
			return nil, err
		}
		sites = append(sites, site)
	}
	return sites, rows.Err()
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(DELETE)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

var errNotFound = errors.New("object not found")

// download はオブジェクトを一時ファイルに保存し、そのパスと ETag を返す。
func (s *Store) download(ctx context.Context) (string, string, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &s.key})
	if err != nil {
		var noKey *types.NoSuchKey
		if errors.As(err, &noKey) {
			return "", "", errNotFound
		}
		return "", "", fmt.Errorf("%s を取得できません: %w", s.Location(), err)
	}
	defer out.Body.Close()

	f, err := os.CreateTemp("", "rss-reader-sites-*.db")
	if err != nil {
		return "", "", err
	}
	if _, err := io.Copy(f, out.Body); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", "", fmt.Errorf("%s を取得できません: %w", s.Location(), err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", "", err
	}
	return f.Name(), aws.ToString(out.ETag), nil
}

// upload は etag が空ならオブジェクトがまだないときだけ、空でなければ etag が一致するときだけ書き込む。
func (s *Store) upload(ctx context.Context, data []byte, etag string) error {
	in := &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &s.key,
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/vnd.sqlite3"),
	}
	if etag == "" {
		in.IfNoneMatch = aws.String("*")
	} else {
		in.IfMatch = aws.String(etag)
	}
	if _, err := s.client.PutObject(ctx, in); err != nil {
		if isConflict(err) {
			return err
		}
		return fmt.Errorf("%s に保存できません: %w", s.Location(), err)
	}
	return nil
}

func isConflict(err error) bool {
	var resp *awshttp.ResponseError
	if !errors.As(err, &resp) {
		return false
	}
	code := resp.HTTPStatusCode()
	return code == http.StatusPreconditionFailed || code == http.StatusConflict
}

func emptyTempFile() (string, error) {
	f, err := os.CreateTemp("", "rss-reader-sites-*.db")
	if err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}
