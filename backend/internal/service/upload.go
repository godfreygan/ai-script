package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/storage"
	"go.uber.org/zap"
)

// UploadService 文件上传服务。把文件转交底层 storage,并返回可访问 URL。
type UploadService struct {
	store storage.Storage
	log   *zap.Logger
}

// NewUploadService 构造。store 为 nil 时所有方法都会返回错误,以便单元测试 / 无对象存储环境跳过。
func NewUploadService(store storage.Storage, log *zap.Logger) *UploadService {
	return &UploadService{store: store, log: log}
}

// UploadResult 单次上传结果。
type UploadResult struct {
	Key  string `json:"key"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
	Type string `json:"type"`
}

// 后端按 namespace 分桶(images / styles / scripts 等)。
var allowedNamespaces = map[string]bool{
	"images": true, "videos": true, "audios": true, "styles": true,
	"covers": true, "scripts": true, "misc": true,
}

// Save 把读入流写到对象存储,key 形如 images/2026/05/12/<rand>.<ext>。
func (s *UploadService) Save(ctx context.Context, namespace, originalName, contentType string, r io.Reader, size int64) (*UploadResult, error) {
	if s.store == nil {
		return nil, errors.New("upload: storage not configured")
	}
	if !allowedNamespaces[namespace] {
		return nil, fmt.Errorf("upload: namespace %q not allowed", namespace)
	}
	ext := strings.ToLower(filepath.Ext(originalName))
	if ext == "" {
		ext = guessExtFromMime(contentType)
	}
	rnd := make([]byte, 8)
	_, _ = rand.Read(rnd)
	now := time.Now()
	key := fmt.Sprintf("%s/%04d/%02d/%02d/%s%s",
		namespace, now.Year(), now.Month(), now.Day(), hex.EncodeToString(rnd), ext)
	url, err := s.store.Put(ctx, key, r, size, contentType)
	if err != nil {
		return nil, err
	}
	return &UploadResult{Key: key, URL: url, Size: size, Type: contentType}, nil
}

func guessExtFromMime(ct string) string {
	switch strings.ToLower(strings.TrimSpace(strings.SplitN(ct, ";", 2)[0])) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "audio/mpeg":
		return ".mp3"
	case "audio/wav":
		return ".wav"
	case "text/plain":
		return ".txt"
	default:
		return ".bin"
	}
}
