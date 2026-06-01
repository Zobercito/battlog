package rotator

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"gobat/internal/config"
	"gobat/internal/utils"
)

var logRe = regexp.MustCompile(`^log_(\d{4})-(\d{2})-\d{2}_\d{2}-\d{2}-\d{2}\.json$`)

// RotateLogs rota y comprime logs según la configuración.
// Se ejecuta al inicio del monitor y del organizador.
func RotateLogs(cfg config.Config) error {
	dirs := []string{cfg.LogsRoot, cfg.LogDir, cfg.MasterDir, cfg.ArchiveDir}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
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

	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := logRe.FindStringSubmatch(entry.Name())
		if len(matches) < 3 {
			continue
		}

		fileYear, fileMonth := matches[1], matches[2]
		// El nombre tiene formato log_YYYY-MM-DD_HH-MM-SS.json → extraer fecha desde la posición 4
		if len(entry.Name()) < 24 {
			continue
		}
		timeStr := entry.Name()[4 : 4+19] // "YYYY-MM-DD_HH-MM-SS"
		fileTime, err := time.Parse("2006-01-02_15-04-05", timeStr)
		if err != nil {
			continue
		}

		if now.Sub(fileTime) > time.Duration(cfg.DiasEnVivo)*24*time.Hour {
			archDir := filepath.Join(cfg.ArchiveDir, fmt.Sprintf("%s-%s", fileYear, fileMonth))
			if err := os.MkdirAll(archDir, 0755); err != nil {
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
				if err := utils.GzipFile(dstPath, gzPath); err != nil {
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
		if !stringsHasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		// Mover master de mes anterior a archive
		monthMatch := regexp.MustCompile(`^master_(\d{4}-\d{2})\.jsonl$`).FindStringSubmatch(entry.Name())
		if monthMatch == nil {
			continue
		}

		archDir := filepath.Join(cfg.ArchiveDir, monthMatch[1])
		if err := os.MkdirAll(archDir, 0755); err != nil {
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

	// Limpiar archivos viejos del archive si RetencionDias > 0
	if cfg.RetencionDias > 0 {
		cleanupOldArchives(cfg)
	}
}

func cleanupOldArchives(cfg config.Config) {
	cutoff := time.Now().Add(-time.Duration(cfg.RetencionDias) * 24 * time.Hour)

	filepath.Walk(cfg.ArchiveDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				log.Printf("Aviso: no se pudo eliminar %s: %v", path, err)
			}
		}
		return nil
	})
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
		if !e.IsDir() && logRe.MatchString(e.Name()) {
			paths = append(paths, filepath.Join(cfg.LogDir, e.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// stringsHasSuffix es un wrapper porque estamos en Go 1.22
func stringsHasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
