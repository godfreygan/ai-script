package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewLocal(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		tmp := t.TempDir()
		s, err := NewLocal(LocalConfig{BaseDir: tmp, PublicHost: "http://localhost:8080/uploads"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s == nil {
			t.Fatal("expected non-nil storage")
		}
	})

	t.Run("empty base dir", func(t *testing.T) {
		_, err := NewLocal(LocalConfig{BaseDir: ""})
		if err == nil {
			t.Fatal("expected error for empty base_dir")
		}
	})

	t.Run("default public host", func(t *testing.T) {
		tmp := t.TempDir()
		s, err := NewLocal(LocalConfig{BaseDir: tmp})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ls := s.(*localStore)
		if ls.publicHost != "/uploads" {
			t.Fatalf("expected default publicHost /uploads, got %q", ls.publicHost)
		}
	})

	t.Run("trims trailing slash", func(t *testing.T) {
		tmp := t.TempDir()
		s, err := NewLocal(LocalConfig{BaseDir: tmp, PublicHost: "http://host/uploads/"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ls := s.(*localStore)
		if ls.publicHost != "http://host/uploads" {
			t.Fatalf("expected trimmed host, got %q", ls.publicHost)
		}
	})
}

func TestSanitizeKey(t *testing.T) {
	cases := []struct {
		key     string
		wantErr bool
	}{
		{"hello.txt", false},
		{"dir/sub/file.png", false},
		{"", true},
		{"../etc/passwd", true},
		{"foo/../../bar", true},
		{"/absolute", true},
	}

	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			k, err := sanitizeKey(c.key)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error for key %q", c.key)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if k == "" {
				t.Fatal("expected non-empty sanitized key")
			}
		})
	}
}

func TestLocalPutGetDelete(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	s, err := NewLocal(LocalConfig{BaseDir: tmp, PublicHost: "http://localhost:8080/uploads"})
	if err != nil {
		t.Fatalf("new local: %v", err)
	}

	key := "test/hello.txt"
	content := "hello world"

	t.Run("put", func(t *testing.T) {
		url, err := s.Put(ctx, key, strings.NewReader(content), int64(len(content)), "text/plain")
		if err != nil {
			t.Fatalf("put: %v", err)
		}
		want := "http://localhost:8080/uploads/test/hello.txt"
		if url != want {
			t.Fatalf("expected url %q, got %q", want, url)
		}
	})

	t.Run("get", func(t *testing.T) {
		rc, err := s.Get(ctx, key)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		defer rc.Close()
		b, _ := io.ReadAll(rc)
		if string(b) != content {
			t.Fatalf("expected %q, got %q", content, string(b))
		}
	})

	t.Run("delete", func(t *testing.T) {
		if err := s.Delete(ctx, key); err != nil {
			t.Fatalf("delete: %v", err)
		}
		_, err := s.Get(ctx, key)
		if !os.IsNotExist(err) {
			t.Fatalf("expected not exist, got %v", err)
		}
	})

	t.Run("delete idempotent", func(t *testing.T) {
		if err := s.Delete(ctx, "nonexistent/key.txt"); err != nil {
			t.Fatalf("delete nonexistent should not error: %v", err)
		}
	})
}

func TestLocalPutInvalidKey(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	s, _ := NewLocal(LocalConfig{BaseDir: tmp, PublicHost: "/uploads"})

	_, err := s.Put(ctx, "../escape.txt", strings.NewReader("x"), 1, "")
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestLocalSignURL(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	s, _ := NewLocal(LocalConfig{BaseDir: tmp, PublicHost: "http://host/uploads"})

	url, err := s.SignURL(ctx, "foo/bar.png", 5*time.Minute)
	if err != nil {
		t.Fatalf("sign url: %v", err)
	}
	if url != "http://host/uploads/foo/bar.png" {
		t.Fatalf("unexpected url: %q", url)
	}

	_, err = s.SignURL(ctx, "", 5*time.Minute)
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestBaseDir(t *testing.T) {
	tmp := t.TempDir()
	s, _ := NewLocal(LocalConfig{BaseDir: tmp})
	if got := BaseDir(s); got != tmp {
		t.Fatalf("expected %q, got %q", tmp, got)
	}
	if got := BaseDir(nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestPublicPrefix(t *testing.T) {
	cases := []struct {
		host   string
		expect string
	}{
		{"http://localhost:8080/uploads", "/uploads"},
		{"https://cdn.example.com/files", "/files"},
		{"/uploads", "/uploads"},
		{"uploads", "/uploads"},
	}

	for _, c := range cases {
		t.Run(c.host, func(t *testing.T) {
			tmp := t.TempDir()
			s, _ := NewLocal(LocalConfig{BaseDir: tmp, PublicHost: c.host})
			got := PublicPrefix(s)
			if got != c.expect {
				t.Fatalf("expected %q, got %q", c.expect, got)
			}
		})
	}
	if got := PublicPrefix(nil); got != "" {
		t.Fatalf("expected empty for nil, got %q", got)
	}
}

func TestFactoryNew(t *testing.T) {
	t.Run("local default", func(t *testing.T) {
		tmp := t.TempDir()
		s, err := New(FactoryConfig{Provider: "", Bucket: tmp, PublicHost: "/uploads"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s == nil {
			t.Fatal("expected non-nil storage")
		}
	})

	t.Run("local explicit", func(t *testing.T) {
		tmp := t.TempDir()
		s, err := New(FactoryConfig{Provider: "local", Bucket: tmp})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s == nil {
			t.Fatal("expected non-nil storage")
		}
	})

	t.Run("aliyun_oss missing endpoint", func(t *testing.T) {
		_, err := New(FactoryConfig{Provider: "aliyun_oss", Bucket: "b", AccessKey: "ak", SecretKey: "sk"})
		if err == nil {
			t.Fatal("expected error for missing endpoint")
		}
	})

	t.Run("tencent_cos missing endpoint", func(t *testing.T) {
		_, err := New(FactoryConfig{Provider: "tencent_cos", Bucket: "b", AccessKey: "ak", SecretKey: "sk"})
		if err == nil {
			t.Fatal("expected error for missing endpoint")
		}
	})

	t.Run("s3 still unimplemented", func(t *testing.T) {
		_, err := New(FactoryConfig{Provider: "s3"})
		if err == nil {
			t.Fatal("expected error for unimplemented provider")
		}
	})

	t.Run("unknown", func(t *testing.T) {
		_, err := New(FactoryConfig{Provider: "unknown"})
		if err == nil {
			t.Fatal("expected error for unknown provider")
		}
	})
}

func TestLocalPutCreatesNestedDir(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	s, _ := NewLocal(LocalConfig{BaseDir: tmp, PublicHost: "/uploads"})

	key := "a/b/c/d.txt"
	_, err := s.Put(ctx, key, strings.NewReader("data"), 4, "")
	if err != nil {
		t.Fatalf("put nested: %v", err)
	}

	full := filepath.Join(tmp, filepath.FromSlash(key))
	if _, err := os.Stat(full); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}

func TestLocalPutCopyError(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	s, _ := NewLocal(LocalConfig{BaseDir: tmp, PublicHost: "/uploads"})

	// Provide a reader that returns an error
	errReader := &errorReader{err: errors.New("boom")}
	_, err := s.Put(ctx, "fail.txt", errReader, 1, "")
	if err == nil {
		t.Fatal("expected error from reader")
	}
}

type errorReader struct {
	err error
}

func (e *errorReader) Read(_ []byte) (int, error) {
	return 0, e.err
}
