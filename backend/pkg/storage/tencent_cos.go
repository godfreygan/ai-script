package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

// TencentCOSConfig 腾讯云 COS 配置
type TencentCOSConfig struct {
	Endpoint   string // 例如 https://bucket-id.cos.ap-guangzhou.myqcloud.com
	Bucket     string // bucket 名称
	AccessKey  string // SecretId
	SecretKey  string // SecretKey
	PublicHost string // 可选，用于生成公共 URL；为空时使用 Endpoint
}

// NewTencentCOS 构造腾讯云 COS Storage 实现
func NewTencentCOS(cfg TencentCOSConfig) (Storage, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("storage: tencent_cos endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("storage: tencent_cos bucket is required")
	}
	if cfg.AccessKey == "" {
		return nil, fmt.Errorf("storage: tencent_cos access_key is required")
	}
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("storage: tencent_cos secret_key is required")
	}

	baseURL, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("storage: tencent_cos parse endpoint %q failed: %w", cfg.Endpoint, err)
	}

	client := cos.NewClient(
		&cos.BaseURL{BucketURL: baseURL},
		&http.Client{
			Transport: &cos.AuthorizationTransport{
				SecretID:  cfg.AccessKey,
				SecretKey: cfg.SecretKey,
			},
		},
	)

	publicHost := strings.TrimRight(cfg.PublicHost, "/")
	if publicHost == "" {
		publicHost = strings.TrimRight(cfg.Endpoint, "/")
	}

	return &tencentCOSStore{
		client:     client,
		bucketName: cfg.Bucket,
		publicHost: publicHost,
	}, nil
}

type tencentCOSStore struct {
	client     *cos.Client
	bucketName string
	publicHost string
}

func (s *tencentCOSStore) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	k, err := sanitizeKey(key)
	if err != nil {
		return "", err
	}

	opts := &cos.ObjectPutOptions{}
	if contentType != "" {
		opts.ObjectPutHeaderOptions = &cos.ObjectPutHeaderOptions{
			ContentType: contentType,
		}
	}

	_, err = s.client.Object.Put(ctx, k, r, opts)
	if err != nil {
		return "", fmt.Errorf("storage: tencent_cos put %q failed: %w", k, err)
	}

	return s.publicHost + "/" + k, nil
}

func (s *tencentCOSStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	k, err := sanitizeKey(key)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Object.Get(ctx, k, nil)
	if err != nil {
		return nil, fmt.Errorf("storage: tencent_cos get %q failed: %w", k, err)
	}
	return resp.Body, nil
}

func (s *tencentCOSStore) Delete(ctx context.Context, key string) error {
	k, err := sanitizeKey(key)
	if err != nil {
		return err
	}

	_, err = s.client.Object.Delete(ctx, k)
	if err != nil {
		return fmt.Errorf("storage: tencent_cos delete %q failed: %w", k, err)
	}
	return nil
}

func (s *tencentCOSStore) SignURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	k, err := sanitizeKey(key)
	if err != nil {
		return "", err
	}

	// 腾讯云 COS 预签名 URL
	presignedURL, err := s.client.Object.GetPresignedURL(ctx, http.MethodGet, k, s.client.GetCredential().SecretID, s.client.GetCredential().SecretKey, expires, nil)
	if err != nil {
		return "", fmt.Errorf("storage: tencent_cos sign url %q failed: %w", k, err)
	}
	return presignedURL.String(), nil
}
