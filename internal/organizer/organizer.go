package organizer

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"gobat/internal/config"
)

// Run inicia el organizador de logs
func Run(cfg config.Config) {
	if err := os.MkdirAll(cfg.LogsRoot, 0755); err != nil {
		log.Printf("ERROR creando directorio base: %v", err)
		return
	}

	releaseLock, err := acquireLock(cfg.LockFile)
	if err != nil {
		log.Printf("ERROR: %v", err)
		return
	}
	defer releaseLock()

	// Cargar archivos ya procesados usando streaming para O(1) memoria
	procesados := make(map[string]bool)
	if f, err := os.Open(cfg.ControlFile); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				procesados[line] = true
			}
		}
		
		// Verificar error de lectura del control
		if err := scanner.Err(); err != nil {
			log.Printf("ERROR CRÍTICO: I/O falló al leer %s: %v", cfg.ControlFile, err)
			f.Close()
			return // Abortar para evitar duplicación masiva
		}
		f.Close()
	}

	// Los logs de sesión se mantienen indefinidamente para máximo histórico.
	// Solo se consolidan en archivos mensuales y se comprimen los antiguos.
	// Listar archivos de log para procesar
	files, err := os.ReadDir(cfg.LogDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("No hay logs de sesión todavía.")
			return
		}
		log.Printf("ERROR leyendo directorio de logs: %v", err)
		return
	}

	re := regexp.MustCompile(`^log_(\d{4})-(\d{2})-\d{2}_\d{2}-\d{2}-\d{2}\.txt$`)

	nuevosProcesados := 0
	omitidos := 0
	erroresProcesamiento := 0

	masterF, err := os.OpenFile(cfg.MasterLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("ERROR abriendo log maestro: %v", err)
		return
	}
	defer func() {
		if err := masterF.Close(); err != nil {
			log.Printf("Aviso: error cerrando log maestro: %v", err)
		}
	}()

	// Abrir archivo de control antes del loop para evitar I/O repetido
	controlF, err := os.OpenFile(cfg.ControlFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("ERROR abriendo archivo de control: %v", err)
		return
	}
	defer controlF.Close()

	// Procesar cada archivo
	for _, file := range files {
		if file.IsDir() || procesados[file.Name()] {
			if !file.IsDir() {
				omitidos++
			}
			continue
		}

		// Comprobar si el monitor tiene el archivo bloqueado
		testLockF, err := os.OpenFile(filepath.Join(cfg.LogDir, file.Name()), os.O_RDONLY, 0)
		if err == nil {
			// Intentamos obtener un lock compartido. Si falla (EWOULDBLOCK), el monitor lo tiene.
			errLock := syscall.Flock(int(testLockF.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
			if errLock != nil {
				testLockF.Close()
				omitidos++
				continue
			}
			syscall.Flock(int(testLockF.Fd()), syscall.LOCK_UN)
			testLockF.Close()
		}

		matches := re.FindStringSubmatch(file.Name())
		if len(matches) < 3 {
			omitidos++
			continue
		}
		_ = matches // año y mes no se usan actualmente

		// Reemplaza la carga masiva en memoria por un streaming línea por línea
		inFile, err := os.Open(filepath.Join(cfg.LogDir, file.Name()))
		if err != nil {
			log.Printf("Aviso: no se pudo abrir log %s: %v", file.Name(), err)
			erroresProcesamiento++
			continue
		}

		// Capturar tamaño original del master para rollback en caso de fallo
		statMaster, err := masterF.Stat()
		if err != nil {
			log.Printf("Aviso: no se pudo obtener stat del log maestro: %v", err)
			inFile.Close()
			erroresProcesamiento++
			continue
		}
		masterOrigSize := statMaster.Size()

		// Procesar y escribir al vuelo
		scanner := bufio.NewScanner(inFile)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024) // Capacidad hasta 1MB por línea
		var writeErr error
		for scanner.Scan() {
			cleanLine := strings.Map(func(r rune) rune {
				if r == 0 || r == '\r' {
					return -1
				}
				return r
			}, scanner.Text())

			if _, err := masterF.WriteString(cleanLine + "\n"); err != nil {
				writeErr = err
				break
			}
		}
		scanErr := scanner.Err()
		inFile.Close()

		var failed bool

		if writeErr != nil {
			log.Printf("Aviso: error de escritura procesando %s: %v", file.Name(), writeErr)
			failed = true
		} else if scanErr != nil {
			log.Printf("ERROR de I/O leyendo log %s: %v", file.Name(), scanErr)
			failed = true
		} else {
			// Si no hubo error, escribimos salto de línea final
			if _, err := masterF.WriteString("\n"); err != nil {
				failed = true
			}

			// Si aún no hay error, hacemos el "Commit" en el archivo de control
			if !failed {
				if _, err := controlF.WriteString(file.Name() + "\n"); err != nil {
					log.Printf("Aviso: no se pudo actualizar control de procesados: %v", err)
					failed = true
				}
			}
		}

		// Rollback: Si cualquier paso falló, revertimos solo el master
		if failed {
			log.Printf("Revirtiendo cambios de %s...", file.Name())
			masterF.Truncate(masterOrigSize)
			erroresProcesamiento++
			continue
		}

		nuevosProcesados++
	}

	fmt.Printf("Procesados %d archivos nuevos, %d omitidos, %d errores.\n", nuevosProcesados, omitidos, erroresProcesamiento)
}

// gzipFile comprime un archivo con gzip de forma segura
func gzipFile(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	tmpDst := dst + ".tmp"
	d, err := os.Create(tmpDst)
	if err != nil {
		return err
	}

	w := gzip.NewWriter(d)
	if _, err := io.Copy(w, f); err != nil {
		w.Close()
		d.Close()
		os.Remove(tmpDst)
		return err
	}

	if err := w.Close(); err != nil {
		d.Close()
		os.Remove(tmpDst)
		return err
	}

	if err := d.Close(); err != nil {
		os.Remove(tmpDst)
		return err
	}

	if err := os.Rename(tmpDst, dst); err != nil {
		os.Remove(tmpDst)
		return err
	}

	return nil
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
	err := syscall.Kill(pid, 0)

	// Solo si el proceso no existe liberamos el lock
	if err != nil && errors.Is(err, syscall.ESRCH) {
		return false
	}

	// Resolución absoluta del ejecutable (agnóstico al nombre del binario)
	myExe, errExe := os.Executable()
	if errExe == nil {
		pidExePath := fmt.Sprintf("/proc/%d/exe", pid)
		pidExe, errLink := os.Readlink(pidExePath)

		// Si leemos el enlace y apunta a otro binario distinto al nuestro, es un PID reciclado
		if errLink == nil && pidExe != myExe {
			return false
		}
		// Si ocurre os.ErrNotExist, el proceso murió justo ahora
		if errors.Is(errLink, os.ErrNotExist) {
			return false
		}
	}

	// Permisos denegados u otros errores, asumimos vivo por seguridad
	return true
}
