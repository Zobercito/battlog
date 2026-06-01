package organizer

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"

	"gobat/internal/config"
	"gobat/internal/processor"
	"gobat/internal/rotator"
)

// Run ejecuta la organización con lock exclusivo (modo independiente)
func Run(cfg config.Config) {
	if err := rotator.RotateLogs(cfg); err != nil {
		log.Printf("ERROR en rotación de logs: %v", err)
	}

	releaseLock, err := acquireLock(cfg.LockFile)
	if err != nil {
		log.Printf("ERROR: %v", err)
		return
	}
	defer releaseLock()

	processor.ProcessSessionLogs(cfg)
}

// acquireLock obtiene un lock exclusivo para el organizador
func acquireLock(lockPath string) (func(), error) {
	for attempt := 0; attempt < 2; attempt++ {
		lockF, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			if _, wErr := lockF.WriteString(strconv.Itoa(os.Getpid())); wErr != nil {
				lockF.Close()
				os.Remove(lockPath)
				return nil, fmt.Errorf("no se pudo escribir lock file: %w", wErr)
			}
			if cErr := lockF.Close(); cErr != nil {
				os.Remove(lockPath)
				return nil, fmt.Errorf("no se pudo cerrar lock file: %w", cErr)
			}
			return func() {
				if err := os.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
					log.Printf("Aviso: no se pudo eliminar lock file: %v", err)
				}
			}, nil
		}

		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("no se pudo crear lock file: %w", err)
		}

		// Lock existente, verificar si el proceso sigue vivo
		pidBytes, rErr := os.ReadFile(lockPath)
		if rErr != nil {
			if errors.Is(rErr, os.ErrNotExist) {
				continue // El lock fue liberado en este exacto milisegundo. Reintentar.
			}
			return nil, fmt.Errorf("lock existente y no legible (%s): %w", lockPath, rErr)
		}

		pidStr := strings.TrimSpace(string(pidBytes))
		pid, pErr := strconv.Atoi(pidStr)
		if pErr != nil || pid <= 0 {
			return nil, fmt.Errorf("lock en estado inconsistente o en creación: %w", pErr)
		}

		if isProcessAlive(pid) {
			return nil, fmt.Errorf("ya hay una instancia corriendo (PID %d)", pid)
		}

		// Limpiar lock huérfano
		if rmErr := os.Remove(lockPath); rmErr != nil {
			return nil, fmt.Errorf("lock huérfano detectado pero no se pudo limpiar: %w", rmErr)
		}
	}

	return nil, fmt.Errorf("no se pudo adquirir lock")
}

// isProcessAlive verifica si un proceso con PID está vivo y es gobat
func isProcessAlive(pid int) bool {
	pidExePath := fmt.Sprintf("/proc/%d/exe", pid)

	// Leer el symlink primero para detectar PID reciclados antes de verificar señal
	pidExe, errLink := os.Readlink(pidExePath)
	if errLink != nil {
		if errors.Is(errLink, os.ErrNotExist) {
			return false // Proceso no existe o PID reciclado
		}
		// Permisos denegados, asumimos vivo por seguridad
		return true
	}

	// Verificar que el binario coincide con el nuestro (PID reciclado = otro binario)
	myExe, errExe := os.Executable()
	if errExe == nil && pidExe != myExe {
		return false
	}

	// Verificar que el proceso sigue vivo con señal 0
	err := syscall.Kill(pid, 0)
	if err != nil && errors.Is(err, syscall.ESRCH) {
		return false
	}

	return true
}
