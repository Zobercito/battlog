package utils

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// GzipFile comprime un archivo con gzip de forma segura (atómica)
func GzipFile(src, dst string) error {
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

// ReadIntFile lee un archivo que contiene un entero
func ReadIntFile(path string) (int64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return 0, fmt.Errorf("archivo vacío: %s", path)
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}

// ReadIntFileToFloat lee un archivo con entero y lo convierte a float dividiendo
func ReadIntFileToFloat(path string, div float64) (float64, error) {
	v, err := ReadIntFile(path)
	if err != nil {
		return 0, err
	}
	return float64(v) / div, nil
}

// ParseSwapUsed extrae el uso de swap de /proc/meminfo
func ParseSwapUsed(data string) (int, bool) {
	var total, free int64
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "SwapTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if v, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
					total = v
				}
			}
		}
		if strings.HasPrefix(line, "SwapFree:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if v, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
					free = v
				}
			}
		}
	}
	if total == 0 {
		return 0, false
	}
	usedKB := total - free
	if usedKB < 0 {
		usedKB = 0
	}
	usedMB := int((usedKB + 512) / 1024)
	return usedMB, true
}
