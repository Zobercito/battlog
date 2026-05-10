package config

import (
	"os"
	"path/filepath"
)

// Config contiene la configuración central de la aplicación
type Config struct {
	LogsRoot          string
	LogDir            string
	HistorialDir      string
	MasterLog         string
	ControlFile       string
	LockFile          string
	IntervaloSegundos int
	MesesSinComprimir int
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
		LogsRoot:          logsRoot,
		LogDir:            filepath.Join(logsRoot, "logs"),
		HistorialDir:      filepath.Join(logsRoot, "logs_historial"),
		MasterLog:         filepath.Join(logsRoot, "logs_todo.txt"),
		ControlFile:       filepath.Join(logsRoot, "archivos_procesados.txt"),
		LockFile:          filepath.Join(logsRoot, ".organizar.lock"),
		IntervaloSegundos: 60,
		MesesSinComprimir: 2,
	}
}
