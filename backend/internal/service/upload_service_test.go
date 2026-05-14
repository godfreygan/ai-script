package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func newUploadService(t *testing.T, putFunc func(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error)) UploadService {
	t.Helper()
	store := &mockStorage{putFunc: putFunc}
	return NewUploadService(store, newNopLog())
}

func TestUploadService_Save(t *testing.T) {
	ctx := context.Background()

	t.Run("normal", func(t *testing.T) {
		s := newUploadService(t, func(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
			return "http://localhost/uploads/" + key, nil
		})
		res, err := s.Save(ctx, "images", "photo.jpg", "image/jpeg", strings.NewReader("hello"), 5)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if res == nil {
			t.Fatal("result is nil")
		}
		if !strings.HasPrefix(res.Key, "images/") {
			t.Fatalf("key=%s want images/ prefix", res.Key)
		}
		if res.URL != "http://localhost/uploads/"+res.Key {
			t.Fatalf("url=%s want http://localhost/uploads/%s", res.URL, res.Key)
		}
		if res.Size != 5 {
			t.Fatalf("size=%d want 5", res.Size)
		}
		if res.Type != "image/jpeg" {
			t.Fatalf("type=%s want image/jpeg", res.Type)
		}
	})

	t.Run("nil store", func(t *testing.T) {
		s := NewUploadService(nil, newNopLog())
		_, err := s.Save(ctx, "images", "photo.jpg", "image/jpeg", strings.NewReader("x"), 1)
		if err == nil {
			t.Fatal("expected error for nil store")
		}
		if err.Error() != "upload: storage not configured" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("invalid namespace", func(t *testing.T) {
		s := newUploadService(t, nil)
		_, err := s.Save(ctx, "hackers", "photo.jpg", "image/jpeg", strings.NewReader("x"), 1)
		if err == nil {
			t.Fatal("expected error for invalid namespace")
		}
		if !strings.Contains(err.Error(), "namespace") {
			t.Fatalf("err=%v want namespace error", err)
		}
	})

	t.Run("file too large", func(t *testing.T) {
		s := newUploadService(t, nil)
		_, err := s.Save(ctx, "images", "photo.jpg", "image/jpeg", strings.NewReader("x"), maxFileSize+1)
		if err == nil {
			t.Fatal("expected error for oversized file")
		}
		if !strings.Contains(err.Error(), "too large") {
			t.Fatalf("err=%v want too large error", err)
		}
	})

	t.Run("disallowed extension", func(t *testing.T) {
		s := newUploadService(t, nil)
		_, err := s.Save(ctx, "images", "photo.exe", "application/octet-stream", strings.NewReader("x"), 1)
		if err == nil {
			t.Fatal("expected error for disallowed extension")
		}
		if !strings.Contains(err.Error(), "not allowed") {
			t.Fatalf("err=%v want not allowed error", err)
		}
	})

	t.Run("empty extension with allowed mime", func(t *testing.T) {
		s := newUploadService(t, func(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
			return "http://localhost/uploads/" + key, nil
		})
		res, err := s.Save(ctx, "images", "photo", "image/jpeg", strings.NewReader("hello"), 5)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !strings.HasSuffix(res.Key, ".jpg") {
			t.Fatalf("key=%s want .jpg suffix", res.Key)
		}
	})

	t.Run("empty extension with disallowed mime", func(t *testing.T) {
		s := newUploadService(t, nil)
		_, err := s.Save(ctx, "images", "photo", "application/zip", strings.NewReader("x"), 1)
		if err == nil {
			t.Fatal("expected error for disallowed mime")
		}
		if !strings.Contains(err.Error(), "not allowed") {
			t.Fatalf("err=%v want not allowed error", err)
		}
	})

	t.Run("storage put error", func(t *testing.T) {
		s := newUploadService(t, func(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
			return "", errors.New("oss timeout")
		})
		_, err := s.Save(ctx, "images", "photo.png", "image/png", strings.NewReader("x"), 1)
		if err == nil {
			t.Fatal("expected error for storage put failure")
		}
		if err.Error() != "oss timeout" {
			t.Fatalf("err=%v want oss timeout", err)
		}
	})

	t.Run("path traversal sanitized", func(t *testing.T) {
		called := false
		s := newUploadService(t, func(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
			called = true
			if strings.Contains(key, "..") || strings.Contains(key, "/etc/") {
				t.Fatalf("key contains path traversal: %s", key)
			}
			return "http://localhost/uploads/" + key, nil
		})
		_, err := s.Save(ctx, "images", "../../../etc/passwd.png", "image/png", strings.NewReader("x"), 1)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !called {
			t.Fatal("storage.Put was not called")
		}
	})

	t.Run("null byte sanitized", func(t *testing.T) {
		called := false
		s := newUploadService(t, func(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
			called = true
			if strings.Contains(key, "\x00") {
				t.Fatalf("key contains null byte")
			}
			return "http://localhost/uploads/" + key, nil
		})
		_, err := s.Save(ctx, "images", "file\x00.png", "image/png", strings.NewReader("x"), 1)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !called {
			t.Fatal("storage.Put was not called")
		}
	})

	t.Run("uppercase extension", func(t *testing.T) {
		s := newUploadService(t, func(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
			return "http://localhost/uploads/" + key, nil
		})
		res, err := s.Save(ctx, "images", "photo.JPG", "image/jpeg", strings.NewReader("hello"), 5)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !strings.HasSuffix(res.Key, ".jpg") {
			t.Fatalf("key=%s want lowercase .jpg suffix", res.Key)
		}
	})

	t.Run("exact max size", func(t *testing.T) {
		s := newUploadService(t, func(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
			return "http://localhost/uploads/" + key, nil
		})
		_, err := s.Save(ctx, "images", "photo.jpg", "image/jpeg", strings.NewReader("x"), maxFileSize)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("all allowed namespaces", func(t *testing.T) {
		for ns := range allowedNamespaces {
			t.Run(ns, func(t *testing.T) {
				s := newUploadService(t, func(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
					return "http://localhost/uploads/" + key, nil
				})
				res, err := s.Save(ctx, ns, "file.txt", "text/plain", strings.NewReader("hello"), 5)
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				if !strings.HasPrefix(res.Key, ns+"/") {
					t.Fatalf("key=%s want %s/ prefix", res.Key, ns)
				}
			})
		}
	})

	t.Run("all allowed extensions", func(t *testing.T) {
		for ext := range allowedExts {
			t.Run(ext, func(t *testing.T) {
				s := newUploadService(t, func(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
					return "http://localhost/uploads/" + key, nil
				})
				_, err := s.Save(ctx, "misc", "file"+ext, "application/octet-stream", strings.NewReader("hello"), 5)
				if err != nil {
					t.Fatalf("unexpected err for ext %s: %v", ext, err)
				}
			})
		}
	})
}

func TestUploadService_guessExtFromMime(t *testing.T) {
	cases := []struct {
		mime string
		want string
	}{
		{"image/jpeg", ".jpg"},
		{"image/png", ".png"},
		{"image/webp", ".webp"},
		{"image/gif", ".gif"},
		{"video/mp4", ".mp4"},
		{"video/webm", ".webm"},
		{"audio/mpeg", ".mp3"},
		{"audio/wav", ".wav"},
		{"text/plain", ".txt"},
		{"application/json", ".bin"},
		{"", ".bin"},
		{"image/jpeg; charset=utf-8", ".jpg"},
		{" IMAGE/JPEG ", ".jpg"},
	}
	for _, c := range cases {
		t.Run(c.mime, func(t *testing.T) {
			got := guessExtFromMime(c.mime)
			if got != c.want {
				t.Fatalf("guessExtFromMime(%q)=%s want %s", c.mime, got, c.want)
			}
		})
	}
}

func TestUploadService_sanitizeFilename(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"photo.jpg", "photo.jpg"},
		{"/etc/passwd.jpg", "passwd.jpg"},
		{"../secret.jpg", "secret.jpg"},
		{"dir\\file.jpg", "file.jpg"},
		{"file\x00.jpg", "file.jpg"},
		{"  spaces.jpg  ", "spaces.jpg"},
		{"", "."},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := sanitizeFilename(c.in)
			if got != c.want {
				t.Fatalf("sanitizeFilename(%q)=%q want %q", c.in, got, c.want)
			}
		})
	}
}
