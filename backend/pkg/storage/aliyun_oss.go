package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// AliyunOSSConfig 阿里云 OSS 配置
type AliyunOSSConfig struct {
	Endpoint   string
	Bucket     string
	AccessKey  string
	SecretKey  string
	PublicHost string // 可选，用于生成公共 URL；为空时使用 Endpoint
}

// NewAliyunOSS 构造阿里云 OSS Storage 实现
func NewAliyunOSS(cfg AliyunOSSConfig) (Storage, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("storage: aliyun_oss endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("storage: aliyun_oss bucket is required")
	}
	if cfg.AccessKey == "" {
		return nil, fmt.Errorf("storage: aliyun_oss access_key is required")
	}
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("storage: aliyun_oss secret_key is required")
	}

	client, err := oss.New(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("storage: aliyun_oss init client failed: %w", err)
	}

	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("storage: aliyun_oss get bucket %q failed: %w", cfg.Bucket, err)
	}

	publicHost := strings.TrimRight(cfg.PublicHost, "/")
	if publicHost == "" {
		publicHost = strings.TrimRight(cfg.Endpoint, "/")
	}

	return &aliyunOSSStore{
		client:     client,
		bucket:     bucket,
		bucketName: cfg.Bucket,
		publicHost: publicHost,
	}, nil
}

type aliyunOSSStore struct {
	client     *oss.Client
	bucket     *oss.Bucket
	bucketName string
	publicHost string
}

func (s *aliyunOSSStore) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	k, err := sanitizeKey(key)
	if err != nil {
		return "", err
	}

	opts := []oss.Option{}
	if contentType != "" {
		opts = append(opts, oss.ContentType(contentType))
	}

	err = s.bucket.PutObject(k, r, opts...)
	if err != nil {
		return "", fmt.Errorf("storage: aliyun_oss put %q failed: %w", k, err)
	}

	return s.publicHost + "/" + k, nil
}

func (s *aliyunOSSStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	k, err := sanitizeKey(key)
	if err != nil {
		return nil, err
	}

	body, err := s.bucket.GetObject(k)
	if err != nil {
		return nil, fmt.Errorf("storage: aliyun_oss get %q failed: %w", k, err)
	}
	return body, nil
}

func (s *aliyunOSSStore) Delete(ctx context.Context, key string) error {
	k, err := sanitizeKey(key)
	if err != nil {
		return err
	}

	err = s.bucket.DeleteObject(k)
	if err != nil {
		return fmt.Errorf("storage: aliyun_oss delete %q failed: %w", k, err)
	}
	return nil
}

func (s *aliyunOSSStore) SignURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	k, err := sanitizeKey(key)
	if err != nil {
		return "", err
	}

	// 阿里云 OSS SignURL 期望的是过期时间戳(秒),而非持续时间
	signedURL, err := s.bucket.SignURL(k, oss.HTTPGet, time.Now().Unix()+int64(expires.Seconds()))
	if err != nil {
		return "", fmt.Errorf("storage: aliyun_oss sign url %q failed: %w", k, err)
	}
	return signedURL, nil
}
