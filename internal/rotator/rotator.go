package rotator

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"battlog/internal/config"
	"battlog/internal/utils"
)

// RotateLogs rota y comprime logs según la configuración.
// Se ejecuta al inicio del monitor y del organizador.
func RotateLogs(cfg config.Config) error {
	dirs := []string{cfg.LogsRoot, cfg.LogDir, cfg.MasterDir, cfg.ArchiveDir}
	for _, d := range dirs {
		if err := os.MkdirAll(d, config.DefaultPermissionDir); err != nil {
			return fmt.Errorf("creando directorio %s: %w", d, err)
		}
	}

	rotateSessionLogs(cfg)
	rotateMasterLog(cfg)

	return nil
}

// rotateSessionLogs mueve logs de sesión viejos de current/ a archive/
func rotateSessionLogs(cfg config.Config) {
	entries, err := os.ReadDir(cfg.LogDir)
	if err != nil {
		log.Printf("Aviso: no se pudo leer %s para rotación: %v", cfg.LogDir, err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := config.LogFileRe.FindStringSubmatch(entry.Name())
		if len(matches) < 7 {
			continue
		}

		fileYear, fileMonth := matches[1], matches[2]
		timeStr := fmt.Sprintf("%s-%s-%s_%s-%s-%s", matches[1], matches[2], matches[3], matches[4], matches[5], matches[6])
		fileTime, err := time.Parse("2006-01-02_15-04-05", timeStr)
		if err != nil || time.Since(fileTime) <= time.Duration(cfg.DiasEnVivo)*24*time.Hour {
			continue
		}

		archDir := filepath.Join(cfg.ArchiveDir, fmt.Sprintf("%s-%s", fileYear, fileMonth))
		if err := os.MkdirAll(archDir, config.DefaultPermissionDir); err != nil {
			log.Printf("Aviso: no se pudo crear %s: %v", archDir, err)
			continue
		}

		srcPath := filepath.Join(cfg.LogDir, entry.Name())
		dstPath := filepath.Join(archDir, entry.Name())

		if err := os.Rename(srcPath, dstPath); err != nil {
			log.Printf("Aviso: no se pudo mover %s a archive: %v", entry.Name(), err)
			continue
		}

		if cfg.ComprimirAlRotar {
			gzPath := dstPath + ".gz"
			if _, err := os.Stat(gzPath); err == nil {
				log.Printf("Aviso: %s ya existe, se omite compresión", filepath.Base(gzPath))
			} else if err := utils.GzipFile(dstPath, gzPath); err != nil {
				log.Printf("Aviso: no se pudo comprimir %s: %v", entry.Name(), err)
				continue
			}
			if err := os.Remove(dstPath); err != nil {
				log.Printf("Aviso: no se pudo eliminar %s tras comprimir: %v", entry.Name(), err)
			}
		}

		log.Printf("Rotado: %s → archive", entry.Name())
	}
}

// rotateMasterLog rota el log maestro por mes si está activo
func rotateMasterLog(cfg config.Config) {
	if !cfg.RotarMaestroPorMes {
		return
	}

	now := time.Now()
	currentMonth := now.Format("2006-01")
	currentMaster := filepath.Join(cfg.MasterDir, fmt.Sprintf("master_%s.jsonl", currentMonth))

	entries, err := os.ReadDir(cfg.MasterDir)
	if err != nil {
		log.Printf("Aviso: no se pudo leer %s: %v", cfg.MasterDir, err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if entry.Name() == filepath.Base(currentMaster) {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		// Mover master de mes anterior a archive
		monthMatch := config.MonthRe.FindStringSubmatch(entry.Name())
		if monthMatch == nil {
			continue
		}

		archDir := filepath.Join(cfg.ArchiveDir, monthMatch[1])
		if err := os.MkdirAll(archDir, config.DefaultPermissionDir); err != nil {
			log.Printf("Aviso: no se pudo crear %s: %v", archDir, err)
			continue
		}

		srcPath := filepath.Join(cfg.MasterDir, entry.Name())
		dstPath := filepath.Join(archDir, entry.Name())

		if cfg.ComprimirAlRotar {
			gzPath := dstPath + ".gz"
			if err := utils.GzipFile(srcPath, gzPath); err != nil {
				log.Printf("Aviso: no se pudo comprimir master %s: %v", entry.Name(), err)
				continue
			}
			if err := os.Remove(srcPath); err != nil {
				log.Printf("Aviso: no se pudo eliminar master %s: %v", entry.Name(), err)
			}
		} else {
			if err := os.Rename(srcPath, dstPath); err != nil {
				log.Printf("Aviso: no se pudo mover master %s: %v", entry.Name(), err)
			}
		}

		log.Printf("Master rotado: %s → archive", entry.Name())
	}

	// Limpiar archivos viejos del archive si RetencionDias > 0.
	// Por defecto RetencionDias=0 lo que significa retención infinita
	// (el diseño es intencionalmente de logging infinito). Solo se limpia
	// cuando el usuario configura explícitamente un valor > 0.
	if cfg.RetencionDias > 0 {
		cleanupOldArchives(cfg)
	}
}

func cleanupOldArchives(cfg config.Config) {
	cutoff := time.Now().Add(-time.Duration(cfg.RetencionDias) * 24 * time.Hour)
	removed := make(map[string]bool)

	filepath.Walk(cfg.ArchiveDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Aviso: error accediendo a %s: %v", path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}

		// Usar la fecha del directorio padre (YYYY-MM) en vez de ModTime
		parentDir := filepath.Base(filepath.Dir(path))
		dirTime, parseErr := time.Parse("2006-01", parentDir)
		if parseErr == nil {
			monthEnd := dirTime.AddDate(0, 1, -1)
			if monthEnd.Before(cutoff) {
				if rmErr := os.Remove(path); rmErr != nil {
					log.Printf("Aviso: no se pudo eliminar %s: %v", path, rmErr)
				} else {
					removed[filepath.Dir(path)] = true
				}
			}
			return nil
		}

		// Fallback a ModTime si el directorio no tiene formato de fecha
		if info.ModTime().Before(cutoff) {
			if rmErr := os.Remove(path); rmErr != nil {
				log.Printf("Aviso: no se pudo eliminar %s: %v", path, rmErr)
			} else {
				removed[filepath.Dir(path)] = true
			}
		}
		return nil
	})

	// Limpiar directorios vacíos después de eliminar archivos
	for dir := range removed {
		if entries, err := os.ReadDir(dir); err == nil && len(entries) == 0 {
			if rmErr := os.Remove(dir); rmErr != nil && !os.IsNotExist(rmErr) {
				log.Printf("Aviso: no se pudo eliminar directorio vacío %s: %v", dir, rmErr)
			}
		}
	}
}

// CurrentMasterPath devuelve la ruta al log maestro del mes actual
func CurrentMasterPath(cfg config.Config) string {
	return filepath.Join(cfg.MasterDir, fmt.Sprintf("master_%s.jsonl", time.Now().Format("2006-01")))
}

// SessionLogPathsSorted devuelve los archivos de sesión ordenados por nombre
func SessionLogPathsSorted(cfg config.Config) ([]string, error) {
	entries, err := os.ReadDir(cfg.LogDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var paths []string
	for _, e := range entries {
		if !e.IsDir() && config.LogFileRe.MatchString(e.Name()) {
			paths = append(paths, filepath.Join(cfg.LogDir, e.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}
