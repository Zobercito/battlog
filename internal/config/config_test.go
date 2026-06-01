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

func TestValidate(t *testing.T) {
	t.Parallel()

	t.Run("valid config", func(t *testing.T) {
		cfg := Config{
			LogsRoot:          "/tmp/test",
			IntervaloSegundos: 60,
			DiasEnVivo:        7,
			RetencionDias:     30,
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("zero interval", func(t *testing.T) {
		cfg := Config{
			IntervaloSegundos: 0,
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for zero interval")
		}
	})

	t.Run("negative interval", func(t *testing.T) {
		cfg := Config{
			IntervaloSegundos: -5,
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for negative interval")
		}
	})

	t.Run("negative DiasEnVivo", func(t *testing.T) {
		cfg := Config{
			IntervaloSegundos: 60,
			DiasEnVivo:        -1,
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for negative DiasEnVivo")
		}
	})

	t.Run("negative RetencionDias", func(t *testing.T) {
		cfg := Config{
			IntervaloSegundos: 60,
			RetencionDias:     -1,
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for negative RetencionDias")
		}
	})

	t.Run("empty LogsRoot", func(t *testing.T) {
		cfg := Config{
			LogsRoot:          "",
			IntervaloSegundos: 60,
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for empty LogsRoot")
		}
	})
}
