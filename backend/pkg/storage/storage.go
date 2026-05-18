// Package storage 对象存储抽象与本地文件实现。
//
// 当生产环境配置阿里云 OSS / 腾讯云 COS / S3 时,可以在此包内添加对应实现。
// 本地实现把文件写入 base_dir,并通过 public_host (如 http://host/uploads/) 暴露访问 URL。
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Storage 对象存储抽象,实现可以是 OSS / COS / S3 / MinIO
type Storage interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (publicURL string, err error)
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	SignURL(ctx context.Context, key string, expires time.Duration) (string, error)
}

// LocalConfig 本地文件存储配置
type LocalConfig struct {
	// BaseDir 文件实际存储根目录(绝对或相对工作目录)
	BaseDir string
	// PublicHost URL 前缀,例如 "http://localhost:8080/uploads",末尾不带 "/"
	PublicHost string
}

// NewLocal 构造一个本地文件存储实现。
func NewLocal(cfg LocalConfig) (Storage, error) {
	if cfg.BaseDir == "" {
		return nil, errors.New("storage: base_dir is required")
	}
	if err := ensureLocalBaseDir(cfg.BaseDir); err != nil {
		return nil, err
	}
	host := strings.TrimRight(cfg.PublicHost, "/")
	if host == "" {
		host = "/uploads"
	}
	return &localStore{baseDir: cfg.BaseDir, publicHost: host}, nil
}

func ensureLocalBaseDir(dir string) error {
	if st, err := os.Stat(dir); err == nil {
		if st.IsDir() {
			return nil
		}
		return fmt.Errorf("storage: base_dir %q exists but is not a directory", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create base dir %q: %w (Docker 只读根文件系统请设置 OSS_BUCKET=/data/uploads 并挂载 uploads_data 卷)", dir, err)
	}
	return nil
}

type localStore struct {
	baseDir    string
	publicHost string
}

func sanitizeKey(key string) (string, error) {
	if key == "" {
		return "", errors.New("storage: empty key")
	}
	// 不允许返回上级路径
	clean := filepath.ToSlash(filepath.Clean(key))
	if strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("storage: invalid key %q", key)
	}
	return clean, nil
}

func (s *localStore) Put(ctx context.Context, key string, r io.Reader, _ int64, _ string) (string, error) {
	k, err := sanitizeKey(key)
	if err != nil {
		return "", err
	}
	full := filepath.Join(s.baseDir, filepath.FromSlash(k))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(full)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return s.publicHost + "/" + k, nil
}

func (s *localStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	k, err := sanitizeKey(key)
	if err != nil {
		return nil, err
	}
	return os.Open(filepath.Join(s.baseDir, filepath.FromSlash(k)))
}

func (s *localStore) Delete(_ context.Context, key string) error {
	k, err := sanitizeKey(key)
	if err != nil {
		return err
	}
	full := filepath.Join(s.baseDir, filepath.FromSlash(k))
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SignURL 本地实现不需要签名,直接返回公共 URL。
func (s *localStore) SignURL(_ context.Context, key string, _ time.Duration) (string, error) {
	k, err := sanitizeKey(key)
	if err != nil {
		return "", err
	}
	return s.publicHost + "/" + k, nil
}

// BaseDir 返回根目录,用于 Gin static route 挂载。
func BaseDir(s Storage) string {
	if ls, ok := s.(*localStore); ok {
		return ls.baseDir
	}
	return ""
}

// PublicPrefix 返回本地实现的对外路径前缀(不带域名 host),用于 Gin Static 挂载。
// 例如 PublicHost="http://host/uploads" -> 返回 "/uploads"。
func PublicPrefix(s Storage) string {
	if ls, ok := s.(*localStore); ok {
		p := ls.publicHost
		// 取最后一个 "/" 之后的为路径,这里直接保留 host 之外的部分
		idx := strings.Index(strings.TrimPrefix(strings.TrimPrefix(p, "http://"), "https://"), "/")
		if idx >= 0 {
			cleaned := strings.TrimPrefix(strings.TrimPrefix(p, "http://"), "https://")
			return "/" + strings.TrimLeft(cleaned[idx:], "/")
		}
		// 已经是相对路径
		if strings.HasPrefix(p, "/") {
			return p
		}
		return "/" + p
	}
	return ""
}
