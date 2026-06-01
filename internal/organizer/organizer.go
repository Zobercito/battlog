package organizer

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"gobat/internal/config"
	"gobat/internal/rotator"
)

// Run ejecuta la organización con lock exclusivo (modo independiente)
func Run(cfg config.Config) {
	// Rotar logs viejos antes de procesar
	if err := rotator.RotateLogs(cfg); err != nil {
		log.Printf("ERROR en rotación de logs: %v", err)
	}

	releaseLock, err := acquireLock(cfg.LockFile)
	if err != nil {
		log.Printf("ERROR: %v", err)
		return
	}
	defer releaseLock()

	processSessionLogs(cfg)
}

// RunWithoutLock ejecuta la organización sin lock (para llamadas internas del monitor)
func RunWithoutLock(cfg config.Config) {
	processSessionLogs(cfg)
}

// processSessionLogs es el core de la organización: consolida sesiones en el log maestro
func processSessionLogs(cfg config.Config) {
	procesados := make(map[string]bool)
	if f, err := os.Open(cfg.ControlFile); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				procesados[line] = true
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("ERROR CRÍTICO: I/O falló al leer %s: %v", cfg.ControlFile, err)
			f.Close()
			return
		}
		f.Close()
	}

	sessionPaths, err := rotator.SessionLogPathsSorted(cfg)
	if err != nil {
		log.Printf("ERROR leyendo directorio de logs: %v", err)
		return
	}

	if len(sessionPaths) == 0 {
		return
	}

	re := regexp.MustCompile(`^log_(\d{4})-(\d{2})-\d{2}_\d{2}-\d{2}-\d{2}\.json$`)

	nuevosProcesados := 0
	omitidos := 0
	erroresProcesamiento := 0

	masterPath := rotator.CurrentMasterPath(cfg)
	masterF, err := os.OpenFile(masterPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("ERROR abriendo log maestro: %v", err)
		return
	}
	defer masterF.Close()

	controlF, err := os.OpenFile(cfg.ControlFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("ERROR abriendo archivo de control: %v", err)
		return
	}
	defer controlF.Close()

	for _, sessionPath := range sessionPaths {
		fileName := filepath.Base(sessionPath)
		if procesados[fileName] {
			omitidos++
			continue
		}

		testLockF, err := os.OpenFile(sessionPath, os.O_RDONLY, 0)
		if err == nil {
			errLock := syscall.Flock(int(testLockF.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
			if errLock != nil {
				testLockF.Close()
				omitidos++
				continue
			}
			syscall.Flock(int(testLockF.Fd()), syscall.LOCK_UN)
			testLockF.Close()
		}

		matches := re.FindStringSubmatch(fileName)
		if len(matches) < 3 {
			omitidos++
			continue
		}

		records, parseErr := parseSessionJSON(sessionPath)
		if parseErr != nil {
			log.Printf("Aviso: no se pudo parsear %s: %v", fileName, parseErr)
			erroresProcesamiento++
			continue
		}
		if len(records) == 0 {
			log.Printf("Aviso: sin registros válidos en %s", fileName)
			erroresProcesamiento++
			continue
		}

		statMaster, err := masterF.Stat()
		if err != nil {
			log.Printf("Aviso: no se pudo obtener stat del log maestro: %v", err)
			erroresProcesamiento++
			continue
		}
		masterOrigSize := statMaster.Size()

		var writeErr error
		for _, record := range records {
			if _, err := masterF.Write(append(record, '\n')); err != nil {
				writeErr = err
				break
			}
		}

		if writeErr != nil {
			log.Printf("Aviso: error de escritura procesando %s: %v", fileName, writeErr)
			masterF.Truncate(masterOrigSize)
			masterF.Seek(masterOrigSize, 0)
			erroresProcesamiento++
			continue
		}

		if _, err := controlF.WriteString(fileName + "\n"); err != nil {
			log.Printf("Aviso: no se pudo actualizar control de procesados: %v", err)
			masterF.Truncate(masterOrigSize)
			masterF.Seek(masterOrigSize, 0)
			erroresProcesamiento++
			continue
		}

		nuevosProcesados++
	}

	if nuevosProcesados > 0 {
		fmt.Printf("Organizados %d archivos nuevos, %d omitidos, %d errores.\n", nuevosProcesados, omitidos, erroresProcesamiento)
	}
}

func parseSessionJSON(path string) ([][]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Records []json.RawMessage `json:"records"`
	}
	if err := json.Unmarshal(data, &wrapper); err == nil {
		result := make([][]byte, 0, len(wrapper.Records))
		for _, r := range wrapper.Records {
			if len(r) > 0 {
				result = append(result, []byte(r))
			}
		}
		return result, nil
	}

	records := parseJSONLines(data)
	if len(records) > 0 {
		return records, nil
	}

	return nil, fmt.Errorf("sin registros parseables")
}

func parseJSONLines(data []byte) [][]byte {
	var records [][]byte
	i := 0
	for i < len(data) {
		line, n := scanLine(data, i)
		i = n
		line = bytes.TrimRight(line, ", \t")
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		if json.Valid(line) {
			records = append(records, append([]byte{}, line...))
		} else {
			parts := splitJSONObjects(line)
			for _, p := range parts {
				if json.Valid(p) {
					records = append(records, append([]byte{}, p...))
				}
			}
		}
	}
	return records
}

func splitJSONObjects(data []byte) [][]byte {
	var parts [][]byte
	depth := 0
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '{' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if data[i] == '}' {
			depth--
			if depth == 0 {
				parts = append(parts, data[start:i+1])
			}
		}
	}
	return parts
}

func scanLine(data []byte, start int) ([]byte, int) {
	for i := start; i < len(data); i++ {
		if data[i] == '\n' {
			return data[start:i], i + 1
		}
	}
	if start < len(data) {
		return data[start:], len(data)
	}
	return nil, len(data)
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
