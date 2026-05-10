package organizer

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gobat/internal/config"
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

	if err := gzipFile(src, dst); err != nil {
		t.Fatalf("gzipFile: %v", err)
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

func TestCompressOldLogsNeverCompressesRecentMonths(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	histDir := filepath.Join(tmpDir, "historial")
	if err := os.MkdirAll(histDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	now := time.Now()
	currentMonth := now.Format("2006-01")
	lastMonth := now.AddDate(0, -1, 0).Format("2006-01")
	oldMonth := now.AddDate(0, -3, 0).Format("2006-01")

	// Crear archivos de prueba
	for _, month := range []string{currentMonth, lastMonth, oldMonth} {
		path := filepath.Join(histDir, month+".txt")
		if err := os.WriteFile(path, []byte("test data"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	cfg := config.Config{HistorialDir: histDir, MesesSinComprimir: 2}
	compressOldLogs(cfg)

	for _, month := range []string{currentMonth, lastMonth} {
		if _, err := os.Stat(filepath.Join(histDir, month+".txt")); err != nil {
			t.Errorf("recent month should not be compressed: %s (%v)", month, err)
		}
		if _, err := os.Stat(filepath.Join(histDir, month+".txt.gz")); !os.IsNotExist(err) {
			t.Errorf("recent month should not have .gz: %s", month)
		}
	}

	gzPath := filepath.Join(histDir, oldMonth+".txt.gz")
	if _, err := os.Stat(gzPath); err != nil {
		t.Fatalf("old month should be compressed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(histDir, oldMonth+".txt")); !os.IsNotExist(err) {
		t.Fatalf("old month source should be removed after compression")
	}
}
