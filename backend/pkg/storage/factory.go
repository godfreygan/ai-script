package storage

import "fmt"

// FactoryConfig 通用工厂配置(从 conf.StorageConfig 平铺过来)。
type FactoryConfig struct {
	Provider   string
	Endpoint   string
	Region     string
	Bucket     string // 本地实现时表示 base_dir
	AccessKey  string
	SecretKey  string
	PublicHost string
}

// New 根据 provider 构造对应 Storage。
// 当前实现的 provider:
//   - "local"  (默认):本地文件存储,bucket -> 目录,public_host -> URL 前缀
//   - "aliyun" / "tencent" / "s3":占位返回错误,后续实现
func New(cfg FactoryConfig) (Storage, error) {
	switch cfg.Provider {
	case "", "local":
		return NewLocal(LocalConfig{BaseDir: cfg.Bucket, PublicHost: cfg.PublicHost})
	case "aliyun", "tencent", "s3", "minio":
		return nil, fmt.Errorf("storage provider %q not implemented yet; please set OSS_PROVIDER=local for now", cfg.Provider)
	default:
		return nil, fmt.Errorf("unknown storage provider %q", cfg.Provider)
	}
}
