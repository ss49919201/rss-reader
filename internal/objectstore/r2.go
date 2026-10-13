package objectstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func isPreconditionFailed(err error) bool {
	var respErr *smithyhttp.ResponseError
	return errors.As(err, &respErr) && respErr.HTTPStatusCode() == 412
}

// R2Config は Cloudflare R2 接続設定。
type R2Config struct {
	AccountID string
	AccessKey string
	SecretKey string
	Bucket    string
}

// R2 は S3 互換 API で R2 に接続する ObjectStore。
type R2 struct {
	client *s3.Client
	bucket string
}

// NewR2FromEnv は環境変数から R2 を構築する。
func NewR2FromEnv() (*R2, error) {
	cfg := R2Config{
		AccountID: os.Getenv("R2_ACCOUNT_ID"),
		AccessKey: os.Getenv("R2_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		Bucket:    os.Getenv("R2_BUCKET"),
	}
	return NewR2(cfg)
}

// NewR2 は R2Config から R2 を構築する。
func NewR2(cfg R2Config) (*R2, error) {
	if cfg.AccountID == "" || cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("R2 の環境変数が不足しています (R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_BUCKET)")
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		UsePathStyle: true,
	})
	return &R2{client: client, bucket: cfg.Bucket}, nil
}

func (r *R2) Get(ctx context.Context, key string) ([]byte, string, error) {
	out, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		var notFound *types.NotFound
		if errors.As(err, &nsk) || errors.As(err, &notFound) {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	defer out.Body.Close()
	body, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, "", err
	}
	etag := ""
	if out.ETag != nil {
		etag = aws.ToString(out.ETag)
	}
	return body, etag, nil
}

func (r *R2) Put(ctx context.Context, key string, body []byte, ifMatchETag string) (string, error) {
	in := &s3.PutObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	}
	if ifMatchETag != "" {
		in.IfMatch = aws.String(ifMatchETag)
	}
	out, err := r.client.PutObject(ctx, in)
	if err != nil {
		if isPreconditionFailed(err) {
			return "", ErrPreconditionFailed
		}
		return "", err
	}
	etag := ""
	if out.ETag != nil {
		etag = aws.ToString(out.ETag)
	}
	return etag, nil
}
