package conf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLocalBucket_RelativeFallback(t *testing.T) {
	got := resolveLocalBucket("local", "/data/uploads")
	if isWritableDir("/data/uploads") {
		if got != "/data/uploads" {
			t.Fatalf("got %q, want /data/uploads", got)
		}
		return
	}
	if got != "./var/uploads" && !isWritableDir(got) {
		t.Fatalf("unexpected bucket dir %q", got)
	}
}

func TestResolveLocalBucket_ExplicitRelative(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "uploads")
	got := resolveLocalBucket("local", dir)
	if got != dir {
		t.Fatalf("got %q, want %q", got, dir)
	}
}

func TestResolveLocalBucket_NonLocalPassthrough(t *testing.T) {
	got := resolveLocalBucket("minio", "ai-script")
	if got != "ai-script" {
		t.Fatalf("got %q", got)
	}
}

func TestIsWritableDir(t *testing.T) {
	dir := t.TempDir()
	if !isWritableDir(dir) {
		t.Fatal("expected writable temp dir")
	}
	_ = os.RemoveAll(dir)
	if isWritableDir(dir) {
		t.Fatal("expected missing dir to be non-writable")
	}
}
