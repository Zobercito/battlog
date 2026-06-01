package rotator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gobat/internal/config"
)

func TestCurrentMasterPath(t *testing.T) {
	cfg := config.Load()
	path := CurrentMasterPath(cfg)
	if path == "" {
		t.Fatal("CurrentMasterPath returned empty")
	}
	if filepath.Dir(path) != cfg.MasterDir {
		t.Fatalf("expected dir %s, got %s", cfg.MasterDir, filepath.Dir(path))
	}
}

func TestSessionLogPathsSortedEmpty(t *testing.T) {
	cfg := config.Load()
	cfg.LogDir = t.TempDir()

	paths, err := SessionLogPathsSorted(cfg)
	if err != nil {
		t.Fatalf("SessionLogPathsSorted: %v", err)
	}
	if len(paths) != 0 {
		t.Fatalf("expected empty, got %d entries", len(paths))
	}
}

func TestRotateLogsCreatesDirs(t *testing.T) {
	baseDir := t.TempDir()
	cfg := config.Load()
	cfg.LogsRoot = baseDir
	cfg.LogDir = filepath.Join(baseDir, "current")
	cfg.MasterDir = filepath.Join(baseDir, "master")
	cfg.ArchiveDir = filepath.Join(baseDir, "archive")
	cfg.DiasEnVivo = 0 // comprimir inmediatamente para tests
	cfg.ComprimirAlRotar = false
	cfg.RotarMaestroPorMes = false

	if err := RotateLogs(cfg); err != nil {
		t.Fatalf("RotateLogs: %v", err)
	}

	for _, d := range []string{cfg.LogDir, cfg.MasterDir, cfg.ArchiveDir} {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			t.Fatalf("directory %s not created", d)
		}
	}
}

func TestRotateLogsMovesOldSession(t *testing.T) {
	baseDir := t.TempDir()
	cfg := config.Load()
	cfg.LogsRoot = baseDir
	cfg.LogDir = filepath.Join(baseDir, "current")
	cfg.MasterDir = filepath.Join(baseDir, "master")
	cfg.ArchiveDir = filepath.Join(baseDir, "archive")
	cfg.DiasEnVivo = 0 // comprimir inmediatamente
	cfg.ComprimirAlRotar = false
	cfg.RotarMaestroPorMes = false

	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Crear un log de sesión de hace 30 días
	oldLogName := "log_2026-04-01_12-00-00.json"
	oldLogPath := filepath.Join(cfg.LogDir, oldLogName)
	if err := os.WriteFile(oldLogPath, []byte(`{"records":[{"ts":1}]}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Poner mtime viejo
	oldTime := time.Now().Add(-30 * 24 * time.Hour)
	os.Chtimes(oldLogPath, oldTime, oldTime)

	if err := RotateLogs(cfg); err != nil {
		t.Fatalf("RotateLogs: %v", err)
	}

	// Verificar que se movió a archive
	archPath := filepath.Join(cfg.ArchiveDir, "2026-04", oldLogName)
	if _, err := os.Stat(archPath); os.IsNotExist(err) {
		t.Fatalf("session log not moved to archive: %s", archPath)
	}
}

func TestStringsHasSuffixIsCorrect(t *testing.T) {
	t.Parallel()

	if !strings.HasSuffix("master_2026-05.jsonl", ".jsonl") {
		t.Fatal("expected true")
	}
	if strings.HasSuffix("master_2026-05.jsonl", ".gz") {
		t.Fatal("expected false")
	}
	if strings.HasSuffix("", ".jsonl") {
		t.Fatal("expected false for empty string")
	}
}
