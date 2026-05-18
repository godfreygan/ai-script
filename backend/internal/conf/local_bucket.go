package conf

import (
	"os"
	"path/filepath"
	"strings"
)

// resolveLocalBucket 为 OSS_PROVIDER=local 选择可写目录。
// 优先使用 OSS_BUCKET；Docker 只读根文件系统下依次尝试 /data/uploads、/tmp/uploads；本地 go run 回退 ./var/uploads。
func resolveLocalBucket(provider, bucket string) string {
	if p := strings.TrimSpace(provider); p != "" && p != "local" {
		return strings.TrimSpace(bucket)
	}

	seen := make(map[string]struct{})
	var candidates []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		candidates = append(candidates, p)
	}
	add(bucket)
	add("/data/uploads")
	add("/tmp/uploads")
	add("./var/uploads")

	for _, dir := range candidates {
		if isRelativePath(dir) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				continue
			}
		}
		if isWritableDir(dir) {
			return dir
		}
	}

	fallback := "./var/uploads"
	_ = os.MkdirAll(fallback, 0o755)
	return fallback
}

func isRelativePath(p string) bool {
	return !strings.HasPrefix(p, "/")
}

func isWritableDir(dir string) bool {
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return false
	}
	f, err := os.Create(filepath.Join(dir, ".write_check"))
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(f.Name())
	return true
}
