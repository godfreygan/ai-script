package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config S3/MinIO 存储配置
type S3Config struct {
	Endpoint   string // S3 兼容端点，例如 http://localhost:9000
	Region     string // AWS 区域，MinIO 可填 "us-east-1"
	Bucket     string // 存储桶名称
	AccessKey  string // 访问密钥 ID
	SecretKey  string // 访问密钥
	PublicHost string // 可选，用于生成公共 URL；为空时使用 Endpoint
}

// NewS3 构造 S3/MinIO Storage 实现
func NewS3(cfg S3Config) (Storage, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("storage: s3 endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("storage: s3 bucket is required")
	}
	if cfg.AccessKey == "" {
		return nil, fmt.Errorf("storage: s3 access_key is required")
	}
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("storage: s3 secret_key is required")
	}

	// 确定是否为 MinIO（非 AWS 端点）
	isMinIO := isMinIOEndpoint(cfg.Endpoint)

	// 配置 AWS SDK
	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
		cfg.AccessKey, cfg.SecretKey, "",
	)))

	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	} else {
		opts = append(opts, config.WithRegion("us-east-1"))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("storage: s3 load config failed: %w", err)
	}

	// 创建 S3 客户端
	s3Opts := []func(*s3.Options){}

	if isMinIO {
		// MinIO 需要路径风格寻址
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	endpoint := normalizeEndpoint(cfg.Endpoint)
	s3Opts = append(s3Opts, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	client := s3.NewFromConfig(awsCfg, s3Opts...)

	// 验证存储桶是否存在
	_, err = client.HeadBucket(context.Background(), &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		return nil, fmt.Errorf("storage: s3 bucket %q not found or access denied: %w", cfg.Bucket, err)
	}

	publicHost := strings.TrimRight(cfg.PublicHost, "/")
	if publicHost == "" {
		publicHost = strings.TrimRight(endpoint, "/") + "/" + cfg.Bucket
	}

	return &s3Store{
		client:     client,
		bucket:     cfg.Bucket,
		publicHost: publicHost,
	}, nil
}

type s3Store struct {
	client     *s3.Client
	bucket     string
	publicHost string
}

func (s *s3Store) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	k, err := sanitizeKey(key)
	if err != nil {
		return "", err
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(k),
		Body:   r,
	}

	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	_, err = s.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("storage: s3 put %q failed: %w", k, err)
	}

	return s.publicHost + "/" + k, nil
}

func (s *s3Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	k, err := sanitizeKey(key)
	if err != nil {
		return nil, err
	}

	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(k),
	})
	if err != nil {
		return nil, fmt.Errorf("storage: s3 get %q failed: %w", k, err)
	}

	return output.Body, nil
}

func (s *s3Store) Delete(ctx context.Context, key string) error {
	k, err := sanitizeKey(key)
	if err != nil {
		return err
	}

	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(k),
	})
	if err != nil {
		return fmt.Errorf("storage: s3 delete %q failed: %w", k, err)
	}

	return nil
}

func (s *s3Store) SignURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	k, err := sanitizeKey(key)
	if err != nil {
		return "", err
	}

	presignClient := s3.NewPresignClient(s.client)

	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(k),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expires
	})
	if err != nil {
		return "", fmt.Errorf("storage: s3 sign url %q failed: %w", k, err)
	}

	return request.URL, nil
}

// isMinIOEndpoint 判断是否为 MinIO 端点
func isMinIOEndpoint(endpoint string) bool {
	endpoint = strings.ToLower(endpoint)
	return strings.Contains(endpoint, "minio") ||
		strings.Contains(endpoint, "localhost") ||
		strings.Contains(endpoint, "127.0.0.1") ||
		!strings.Contains(endpoint, "amazonaws.com")
}

// normalizeEndpoint 标准化端点 URL
func normalizeEndpoint(endpoint string) string {
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}
	return strings.TrimRight(endpoint, "/")
}
