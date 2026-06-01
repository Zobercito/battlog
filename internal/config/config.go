package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// LogFileRe matches session log filenames and captures all date components
var LogFileRe = regexp.MustCompile(`^log_(\d{4})-(\d{2})-(\d{2})_(\d{2})-(\d{2})-(\d{2})\.json$`)

// MonthRe matches master log filenames by month
var MonthRe = regexp.MustCompile(`^master_(\d{4}-\d{2})\.jsonl$`)

const (
	DefaultPermissionDir  os.FileMode = 0755
	DefaultPermissionFile os.FileMode = 0644
)

// Config contiene la configuración central de la aplicación
type Config struct {
	LogsRoot                 string
	LogDir                   string
	MasterDir                string
	ArchiveDir               string
	ControlFile              string
	LockFile                 string
	IntervaloSegundos        int
	OrganizarCadaIteraciones int  // cada cuántas iteraciones organizar (0 = nunca)
	DiasEnVivo               int  // días antes de comprimir logs de sesión
	ComprimirAlRotar         bool // comprimir logs al mover a archive
	RotarMaestroPorMes       bool // rotar log maestro por mes
	RetencionDias            int    // 0 = infinito
	MaxBatteryCycles         int    // ciclos máximos estimados de la batería
	LockFilePath             string // ruta al archivo de lock
}

// Load carga la configuración con valores por defecto.
// El diseño es intencionalmente de logging infinito: los logs rotan a archive/ y se comprimen,
// pero nunca se eliminan a menos que RetencionDias > 0. Esto permite trazabilidad completa.
func Load() Config {
	logsRoot := os.Getenv("GOBAT_LOG_DIR")
	if logsRoot == "" {
		baseDir := "."
		if exe, err := os.Executable(); err == nil {
			baseDir = filepath.Dir(exe)
		} else if wd, err := os.Getwd(); err == nil {
			baseDir = wd
		}
		logsRoot = filepath.Join(baseDir, "logs")
	}

	cfg := Config{
		LogsRoot:                 logsRoot,
		LogDir:                   filepath.Join(logsRoot, "current"),
		MasterDir:                filepath.Join(logsRoot, "master"),
		ArchiveDir:               filepath.Join(logsRoot, "archive"),
		ControlFile:              filepath.Join(logsRoot, "archivos_procesados.txt"),
		LockFile:                 filepath.Join(logsRoot, ".organizar.lock"),
		IntervaloSegundos:        60,
		OrganizarCadaIteraciones: 60,
		DiasEnVivo:               7,
		ComprimirAlRotar:         true,
		RotarMaestroPorMes:       true,
		RetencionDias:            0,
		MaxBatteryCycles:         1000,
	}

	if v := os.Getenv("GOBAT_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.IntervaloSegundos = n
		}
	}
	if v := os.Getenv("GOBAT_DIAS_EN_VIVO"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.DiasEnVivo = n
		}
	}
	if v := os.Getenv("GOBAT_RETENCION_DIAS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.RetencionDias = n
		}
	}
	if v := os.Getenv("GOBAT_MAX_CYCLES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxBatteryCycles = n
		}
	}
	if v := os.Getenv("GOBAT_COMPRIMIR"); v != "" {
		cfg.ComprimirAlRotar = v == "true"
	}

	return cfg
}

// Validate verifica que la configuración sea coherente
func (c Config) Validate() error {
	if c.IntervaloSegundos <= 0 {
		return fmt.Errorf("IntervaloSegundos debe ser > 0, got %d", c.IntervaloSegundos)
	}
	if c.DiasEnVivo < 0 {
		return fmt.Errorf("DiasEnVivo no puede ser negativo, got %d", c.DiasEnVivo)
	}
	if c.RetencionDias < 0 {
		return fmt.Errorf("RetencionDias no puede ser negativo, got %d", c.RetencionDias)
	}
	if c.LogsRoot == "" {
		return fmt.Errorf("LogsRoot no puede estar vacío")
	}
	return nil
}
