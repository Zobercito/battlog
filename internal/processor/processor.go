package processor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"gobat/internal/config"
	"gobat/internal/rotator"
)

// ProcessSessionLogs consolida sesiones en el log maestro (core compartido)
func ProcessSessionLogs(cfg config.Config) {
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

	nuevosProcesados := 0
	omitidos := 0
	erroresProcesamiento := 0

	masterPath := rotator.CurrentMasterPath(cfg)
	masterF, err := os.OpenFile(masterPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, config.DefaultPermissionFile)
	if err != nil {
		log.Printf("ERROR abriendo log maestro: %v", err)
		return
	}
	defer masterF.Close()

	if err := syscall.Flock(int(masterF.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		log.Printf("Aviso: otro proceso está procesando logs, se omite esta ejecución: %v", err)
		return
	}
	defer syscall.Flock(int(masterF.Fd()), syscall.LOCK_UN)

	controlF, err := os.OpenFile(cfg.ControlFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, config.DefaultPermissionFile)
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

		var rawData []byte
		testLockF, err := os.OpenFile(sessionPath, os.O_RDONLY, 0)
		if err == nil {
			errLock := syscall.Flock(int(testLockF.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
			if errLock != nil {
				testLockF.Close()
				omitidos++
				continue
			}
			rawData, err = io.ReadAll(testLockF)
			syscall.Flock(int(testLockF.Fd()), syscall.LOCK_UN)
			testLockF.Close()
			if err != nil {
				log.Printf("Aviso: no se pudo leer %s: %v", fileName, err)
				omitidos++
				continue
			}
		} else {
			// Si no podemos abrir, leer via ReadFile (sin lock)
			rawData, err = os.ReadFile(sessionPath)
			if err != nil {
				log.Printf("Aviso: no se pudo leer %s: %v", fileName, err)
				omitidos++
				continue
			}
		}

		matches := config.LogFileRe.FindStringSubmatch(fileName)
		if len(matches) < 3 {
			omitidos++
			continue
		}

		records, parseErr := parseSessionJSON(rawData)
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

func parseSessionJSON(data []byte) ([][]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("archivo vacío")
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

// splitJSONObjects separa objetos JSON respetando strings (para no romper por
// llaves dentro de valores string como {"name":"test}value"}).
func splitJSONObjects(data []byte) [][]byte {
	var parts [][]byte
	depth := 0
	start := 0
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if data[i] == '\\' {
				escaped = true
				continue
			}
			if data[i] == '"' {
				inString = false
			}
			continue
		}
		if data[i] == '"' {
			inString = true
			continue
		}
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
