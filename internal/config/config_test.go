package config

import (
	"testing"
)

func TestLoadReturnsConfig(t *testing.T) {
	cfg := Load()
	if cfg.LogsRoot == "" {
		t.Fatal("LogsRoot should not be empty")
	}
	if cfg.LogDir == "" {
		t.Fatal("LogDir should not be empty")
	}
	if cfg.MasterDir == "" {
		t.Fatal("MasterDir should not be empty")
	}
	if cfg.ArchiveDir == "" {
		t.Fatal("ArchiveDir should not be empty")
	}
	if cfg.IntervaloSegundos != 60 {
		t.Fatalf("expected 60, got %d", cfg.IntervaloSegundos)
	}
	if cfg.DiasEnVivo != 7 {
		t.Fatalf("expected 7, got %d", cfg.DiasEnVivo)
	}
	if !cfg.ComprimirAlRotar {
		t.Fatal("ComprimirAlRotar should be true")
	}
	if !cfg.RotarMaestroPorMes {
		t.Fatal("RotarMaestroPorMes should be true")
	}
	if cfg.RetencionDias != 0 {
		t.Fatalf("expected 0, got %d", cfg.RetencionDias)
	}
}

func TestConfigPathsAbsolute(t *testing.T) {
	cfg := Load()
	if cfg.LogDir == cfg.LogsRoot {
		t.Fatal("LogDir should differ from LogsRoot")
	}
	if cfg.MasterDir == cfg.LogsRoot {
		t.Fatal("MasterDir should differ from LogsRoot")
	}
	if cfg.ArchiveDir == cfg.LogsRoot {
		t.Fatal("ArchiveDir should differ from LogsRoot")
	}
}
