package utils

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestReadIntFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("42\n"), 0644); err != nil {
		t.Fatal(err)
	}
	v, err := ReadIntFile(path)
	if err != nil {
		t.Fatalf("ReadIntFile: %v", err)
	}
	if v != 42 {
		t.Fatalf("expected 42, got %d", v)
	}
}

func TestReadIntFileEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadIntFile(path)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestReadIntFileToFloat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "temp.txt")
	if err := os.WriteFile(path, []byte("42000\n"), 0644); err != nil {
		t.Fatal(err)
	}
	v, err := ReadIntFileToFloat(path, 1000.0)
	if err != nil {
		t.Fatalf("ReadIntFileToFloat: %v", err)
	}
	if v != 42.0 {
		t.Fatalf("expected 42.0, got %f", v)
	}
}

func TestParseSwapUsed(t *testing.T) {
	t.Parallel()

	data := `SwapTotal:       2097152 kB
SwapFree:        1048576 kB`
	used, ok := ParseSwapUsed(data)
	if !ok {
		t.Fatal("expected ok")
	}
	if used != 1024 {
		t.Fatalf("expected 1024 MB, got %d", used)
	}
}

func TestParseSwapUsedZero(t *testing.T) {
	t.Parallel()

	data := `SwapTotal:       0 kB
SwapFree:        0 kB`
	_, ok := ParseSwapUsed(data)
	if ok {
		t.Fatal("expected false for zero swap")
	}
}

func TestParseSwapUsedNegative(t *testing.T) {
	t.Parallel()

	data := `SwapTotal:       1024 kB
SwapFree:        2048 kB`
	used, ok := ParseSwapUsed(data)
	if !ok {
		t.Fatal("expected ok")
	}
	if used != 0 {
		t.Fatalf("expected 0, got %d", used)
	}
}

func TestGzipFileCreatesValidArchive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "source.txt.gz")
	content := "linea 1\nlinea 2\n"

	if err := os.WriteFile(src, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := GzipFile(src, dst); err != nil {
		t.Fatalf("GzipFile: %v", err)
	}

	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("gzip output file not created: %v", err)
	}

	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source file should still exist: %v", err)
	}

	gzf, err := os.Open(dst)
	if err != nil {
		t.Fatalf("os.Open gzip file: %v", err)
	}
	defer gzf.Close()

	reader, err := gzip.NewReader(gzf)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}

	if got := buf.String(); got != content {
		t.Fatalf("gzip content mismatch: got %q, want %q", got, content)
	}
}
