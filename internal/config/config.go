package config

import (
	"os"
	"path/filepath"
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
	RetencionDias            int  // 0 = infinito
}

// Load carga la configuración con valores por defecto
func Load() Config {
	baseDir := "."
	if exe, err := os.Executable(); err == nil {
		baseDir = filepath.Dir(exe)
	} else if wd, err := os.Getwd(); err == nil {
		baseDir = wd
	}

	logsRoot := filepath.Join(baseDir, "logs")
	return Config{
		LogsRoot:                 logsRoot,
		LogDir:                   filepath.Join(logsRoot, "current"),
		MasterDir:                filepath.Join(logsRoot, "master"),
		ArchiveDir:               filepath.Join(logsRoot, "archive"),
		ControlFile:              filepath.Join(logsRoot, "archivos_procesados.txt"),
		LockFile:                 filepath.Join(logsRoot, ".organizar.lock"),
		IntervaloSegundos:        60,
		OrganizarCadaIteraciones: 60, // cada 60 iteraciones ≈ 1 hora con intervalo de 60s
		DiasEnVivo:               7,
		ComprimirAlRotar:         true,
		RotarMaestroPorMes:       true,
		RetencionDias:            0,
	}
}
