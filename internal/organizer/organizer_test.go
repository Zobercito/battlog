package organizer

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"gobat/internal/utils"
)

func TestGzipFileCreatesValidArchive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "source.txt.gz")
	content := "linea 1\nlinea 2\n"

	if err := os.WriteFile(src, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := utils.GzipFile(src, dst); err != nil {
		t.Fatalf("GzipFile: %v", err)
	}

	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("gzip output file not created: %v", err)
	}

	// Verificar que la fuente se mantiene (gzipFile no la elimina)
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source file should still exist: %v", err)
	}

	// Verificar contenido descomprimido
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
