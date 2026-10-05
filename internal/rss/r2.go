package rss

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

const (
	// DefaultBucket はサイト一覧を置く R2 バケットの既定名。R2_BUCKET で上書きできる。
	DefaultBucket = "rss-reader-sites"
	// SitesObjectKey はバケット内の SQLite ファイル。R2_OBJECT_KEY で上書きできる。
	SitesObjectKey = "sites.db"

	maxDBBytes = 32 << 20
)

var accountIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{32}$`)

// R2Config は sites.db を読み書きする Cloudflare R2 の接続情報。
type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	ObjectKey       string
	Endpoint        string
}

// ConfigFromEnv は次の環境変数を読む。
//
//	R2_ACCOUNT_ID          Cloudflare アカウント ID（32桁の16進数）
//	R2_ACCESS_KEY_ID       R2 API トークンの Access Key ID
//	R2_SECRET_ACCESS_KEY   R2 API トークンの Secret Access Key
//	R2_BUCKET              バケット名。空なら rss-reader-sites
//	R2_OBJECT_KEY          オブジェクトキー。空なら sites.db
//	R2_ENDPOINT            任意。空なら https://<ACCOUNT_ID>.r2.cloudflarestorage.com
func ConfigFromEnv() (R2Config, error) {
	cfg := R2Config{
		AccountID:       strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID")),
		AccessKeyID:     strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")),
		SecretAccessKey: strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY")),
		Bucket:          strings.TrimSpace(os.Getenv("R2_BUCKET")),
		ObjectKey:       strings.TrimSpace(os.Getenv("R2_OBJECT_KEY")),
		Endpoint:        strings.TrimSpace(os.Getenv("R2_ENDPOINT")),
	}
	if cfg.Bucket == "" {
		cfg.Bucket = DefaultBucket
	}
	if cfg.ObjectKey == "" {
		cfg.ObjectKey = SitesObjectKey
	}
	if err := cfg.validate(); err != nil {
		return R2Config{}, err
	}
	return cfg, nil
}

func (cfg R2Config) validate() error {
	var missing []string
	if cfg.AccessKeyID == "" {
		missing = append(missing, "R2_ACCESS_KEY_ID")
	}
	if cfg.SecretAccessKey == "" {
		missing = append(missing, "R2_SECRET_ACCESS_KEY")
	}
	if cfg.Endpoint == "" && cfg.AccountID == "" {
		missing = append(missing, "R2_ACCOUNT_ID")
	}
	if len(missing) > 0 {
		return fmt.Errorf("R2の設定が不足しています: %s", strings.Join(missing, ", "))
	}
	if !validToken(cfg.AccessKeyID) {
		return fmt.Errorf("R2_ACCESS_KEY_ID が不正です")
	}
	if !validToken(cfg.SecretAccessKey) {
		return fmt.Errorf("R2_SECRET_ACCESS_KEY が不正です")
	}
	if cfg.AccountID != "" && !accountIDPattern.MatchString(cfg.AccountID) {
		return fmt.Errorf("R2_ACCOUNT_ID は32桁の16進数にしてください")
	}
	if err := validateBucket(cfg.Bucket); err != nil {
		return err
	}
	if err := validateObjectKey(cfg.ObjectKey); err != nil {
		return err
	}
	if cfg.Endpoint != "" {
		if _, err := normalizeEndpoint(cfg.Endpoint); err != nil {
			return err
		}
	}
	return nil
}

func validToken(s string) bool {
	if len(s) < 8 || len(s) > 256 {
		return false
	}
	for _, r := range s {
		if r <= ' ' || r >= 0x7f {
			return false
		}
	}
	return true
}

func validateLocation(bucket, key string) error {
	if err := validateBucket(bucket); err != nil {
		return err
	}
	return validateObjectKey(key)
}

func validateBucket(name string) error {
	if len(name) < 3 || len(name) > 63 {
		return fmt.Errorf("R2バケット名が不正です: %q", name)
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '-':
			if i == 0 || i == len(name)-1 {
				return fmt.Errorf("R2バケット名が不正です: %q", name)
			}
		default:
			return fmt.Errorf("R2バケット名が不正です: %q", name)
		}
	}
	return nil
}

func validateObjectKey(key string) error {
	if key == "" || len(key) > 1024 || strings.HasPrefix(key, "/") || strings.HasSuffix(key, "/") {
		return fmt.Errorf("R2オブジェクトキーが不正です: %q", key)
	}
	if strings.Contains(key, `\`) || strings.Contains(key, "//") {
		return fmt.Errorf("R2オブジェクトキーが不正です: %q", key)
	}
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("R2オブジェクトキーが不正です: %q", key)
		}
	}
	return nil
}

func normalizeEndpoint(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil {
		return "", fmt.Errorf("R2_ENDPOINT が不正です")
	}
	switch u.Scheme {
	case "https":
	case "http":
		host := u.Hostname()
		if host != "127.0.0.1" && host != "localhost" {
			return "", fmt.Errorf("R2_ENDPOINT は https にしてください")
		}
	default:
		return "", fmt.Errorf("R2_ENDPOINT が不正です")
	}
	if u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", fmt.Errorf("R2_ENDPOINT はオリジンだけにしてください")
	}
	u.Path = ""
	u.RawPath = ""
	return u.String(), nil
}

func (cfg R2Config) endpoint() (string, error) {
	if cfg.Endpoint != "" {
		return normalizeEndpoint(cfg.Endpoint)
	}
	if !accountIDPattern.MatchString(cfg.AccountID) {
		return "", fmt.Errorf("R2_ACCOUNT_ID は32桁の16進数にしてください")
	}
	return "https://" + strings.ToLower(cfg.AccountID) + ".r2.cloudflarestorage.com", nil
}

// R2 は S3 互換 API で sites.db を読み書きする。
type R2 struct {
	client *s3.Client
}

// NewR2 は設定から R2 クライアントを作る。通信は Get / Put まで行わない。
func NewR2(cfg R2Config) (*R2, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	endpoint, err := cfg.endpoint()
	if err != nil {
		return nil, err
	}
	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		// パス形式: https://<account>.r2.cloudflarestorage.com/<bucket>/<key>
		UsePathStyle: true,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		// R2 は SDK 既定のチェックサム（CRC32）と相性が悪い。必須の操作だけ計算する。
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	return &R2{client: client}, nil
}

// Get はオブジェクトを読む。無いときは errNotFound を返す。
func (r *R2) Get(ctx context.Context, bucket, key string) ([]byte, error) {
	out, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, errNotFound
		}
		return nil, fmt.Errorf("R2から %s/%s を読めません: %w", bucket, key, err)
	}
	defer out.Body.Close()
	data, err := io.ReadAll(io.LimitReader(out.Body, maxDBBytes+1))
	if err != nil {
		return nil, fmt.Errorf("R2から %s/%s を読めません: %w", bucket, key, err)
	}
	if len(data) > maxDBBytes {
		return nil, fmt.Errorf("R2上の %s/%s が大きすぎます", bucket, key)
	}
	return data, nil
}

// Put はオブジェクトを置き換える。
func (r *R2) Put(ctx context.Context, bucket, key string, data []byte) error {
	if len(data) == 0 || len(data) > maxDBBytes {
		return fmt.Errorf("書き込むデータベースの大きさが不正です")
	}
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   aws.String("application/vnd.sqlite3"),
	})
	if err != nil {
		return fmt.Errorf("R2へ %s/%s を書けません: %w", bucket, key, err)
	}
	return nil
}

func isNotFound(err error) bool {
	var api smithy.APIError
	if !errors.As(err, &api) {
		return false
	}
	// バケット自体が無い NoSuchBucket は欠落オブジェクトと区別する。
	switch api.ErrorCode() {
	case "NoSuchKey", "NotFound":
		return true
	default:
		return false
	}
}
