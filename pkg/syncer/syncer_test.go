package syncer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestFileSyncer_CopyAndVerify(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	writeFile(t, src, "hello world")

	fs := New(false, true) // verify enabled
	if _, err := fs.Execute(SyncTask{Type: "copy", SrcPath: src, DstPath: dst}); err != nil {
		t.Fatalf("copy+verify failed: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != "hello world" {
		t.Fatalf("dst content = %q, want %q", got, "hello world")
	}
}

func TestFileSyncer_ConflictSkip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	writeFile(t, src, "new-content")
	writeFile(t, dst, "old-content")

	// Make dst newer than src so it is treated as a conflict.
	old := time.Now().Add(-time.Hour)
	newer := time.Now()
	if err := os.Chtimes(src, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(dst, newer, newer); err != nil {
		t.Fatal(err)
	}

	fs := New(false, false)
	fs.SetConflictDetection(true)
	fs.SetConflictResolution("skip")

	if _, err := fs.Execute(SyncTask{Type: "copy", SrcPath: src, DstPath: dst}); err != nil {
		t.Fatalf("conflict skip returned error: %v", err)
	}

	got, _ := os.ReadFile(dst)
	if string(got) != "old-content" {
		t.Fatalf("dst was overwritten; conflict skip did not work (got %q)", got)
	}
}

func TestFileSyncer_Backup(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	writeFile(t, src, "v2")
	writeFile(t, dst, "v1")

	fs := New(true, false) // backup enabled
	if _, err := fs.Execute(SyncTask{Type: "copy", SrcPath: src, DstPath: dst}); err != nil {
		t.Fatalf("copy failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "dst.txt.") && strings.HasSuffix(e.Name(), ".bak") {
			found = true
		}
	}
	if !found {
		t.Fatal("backup file was not created")
	}
}
