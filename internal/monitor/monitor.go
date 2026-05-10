package monitor

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gobat/internal/config"
	"gobat/internal/system"
)

// Run inicia el monitor de batería
func Run(cfg config.Config) {
	batPath, err := system.DetectBattery()
	if err != nil {
		log.Printf("ERROR: %v", err)
		return
	}

	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		log.Printf("Error creando directorio de logs: %v", err)
		return
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFileName := filepath.Join(cfg.LogDir, fmt.Sprintf("log_%s.txt", timestamp))

	f, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Error creando log: %v", err)
		return
	}
	defer f.Close()

	// Bloqueamos el archivo a nivel SO. Se libera solo al hacer Close() o si el proceso muere.
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		log.Printf("Aviso: no se pudo aplicar flock al archivo activo: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	if _, err := fmt.Fprintf(f, "--- Inicio del Log: %s ---\n", time.Now().Format(time.RFC1123)); err != nil {
		log.Printf("Error escribiendo cabecera de log: %v", err)
		return
	}
	if _, err := fmt.Fprintf(f, "--- Dispositivo: %s ---\n", batPath); err != nil {
		log.Printf("Error escribiendo cabecera de log: %v", err)
		return
	}
	if _, err := fmt.Fprintf(f, "--- Intervalo: %ds ---\n\n", cfg.IntervaloSegundos); err != nil {
		log.Printf("Error escribiendo cabecera de log: %v", err)
		return
	}
	if err := f.Sync(); err != nil {
		log.Printf("Aviso: no se pudo sincronizar cabecera de log: %v", err)
	}

	fmt.Printf("Monitoreando batería %s cada %ds...\n", batPath, cfg.IntervaloSegundos)

	ticker := time.NewTicker(time.Duration(cfg.IntervaloSegundos) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			body := buildLogEntry(batPath)
			entry := fmt.Sprintf("--- %s ---\n%s\n\n", time.Now().Format("2006-01-02 15:04:05"), body)
			if _, err := f.WriteString(entry); err != nil {
				log.Printf("Error escribiendo entrada de log: %v", err)
				continue
			}
			if err := f.Sync(); err != nil {
				log.Printf("Aviso: no se pudo sincronizar el log: %v", err)
			}
		case <-sigCh:
			if _, err := fmt.Fprintf(f, "\n--- Fin del Log: %s ---\n", time.Now().Format(time.RFC1123)); err != nil {
				log.Printf("Error escribiendo cierre de log: %v", err)
			}
			return
		}
	}
}

// buildLogEntry construye una entrada completa del log con información de sistema
func buildLogEntry(batPath string) string {
	bat := system.GetBatteryInfo(batPath)
	sys := system.GetSystemInfo()

	var b strings.Builder

	// Battery Status Section
	b.WriteString("Battery Status:\n")
	writeKV := func(key, display string) {
		if v, ok := bat[key]; ok && v != "" {
			b.WriteString(fmt.Sprintf("  %-24s : %s\n", display, v))
		} else {
			b.WriteString(fmt.Sprintf("  %-24s : %s\n", display, "N/A"))
		}
	}

	writeKV("state", "state")
	writeKV("percentage", "percentage")
	writeKV("energy-rate", "energy-rate")
	writeKV("voltage", "voltage")

	if v, ok := bat["time to full"]; ok && v != "" {
		b.WriteString(fmt.Sprintf("  %-24s : %s\n", "time to full", v))
	}
	if v, ok := bat["time to empty"]; ok && v != "" {
		b.WriteString(fmt.Sprintf("  %-24s : %s\n", "time to empty", v))
	}

	if v, ok := sys["power_profile"]; ok {
		b.WriteString(fmt.Sprintf("  %-24s : %s\n", "power_profile", v))
	}

	// Battery Health Section
	b.WriteString("\nBattery Health:\n")
	writeKV("charge-cycles", "cycles")
	writeKV("energy-full", "capacity (current)")
	writeKV("energy-full-design", "capacity (design)")
	writeKV("wear_level", "wear_level")

	// CPU Section
	b.WriteString("\nCPU:\n")
	cpuKeys := []string{"current_frequency", "cpu_temp", "load_avg (1min)", "thermal_throttling"}
	for _, k := range cpuKeys {
		if v, ok := sys[k]; ok && v != "" {
			b.WriteString(fmt.Sprintf("  %-24s : %s\n", k, v))
		} else {
			b.WriteString(fmt.Sprintf("  %-24s : %s\n", k, "N/A"))
		}
	}

	// GPU Section
	b.WriteString("\nGPU:\n")
	if v, ok := sys["nvidia_gpu"]; ok {
		b.WriteString(fmt.Sprintf("  %-24s : %s\n", "nvidia_gpu", v))
	} else {
		b.WriteString(fmt.Sprintf("  %-24s : %s\n", "nvidia_gpu", "N/A"))
	}

	// Memory Section
	b.WriteString("\nMemory:\n")
	if v, ok := sys["used"]; ok {
		b.WriteString(fmt.Sprintf("  %-24s : %s\n", "used", v))
	} else {
		b.WriteString(fmt.Sprintf("  %-24s : %s\n", "used", "N/A"))
	}
	if v, ok := sys["swap_used"]; ok {
		b.WriteString(fmt.Sprintf("  %-24s : %s\n", "swap_used", v))
	} else {
		b.WriteString(fmt.Sprintf("  %-24s : %s\n", "swap_used", "N/A"))
	}

	// Display Section
	b.WriteString("\nDisplay:\n")
	if v, ok := sys["brightness"]; ok {
		b.WriteString(fmt.Sprintf("  %-24s : %s\n", "brightness", v))
	} else {
		b.WriteString(fmt.Sprintf("  %-24s : %s\n", "brightness", "N/A"))
	}

	// Top Processes Section
	b.WriteString("\nTop Processes (cpu):\n")
	for i := 1; i <= 8; i++ {
		key := fmt.Sprintf("top%d", i)
		if v, ok := sys[key]; ok {
			b.WriteString(fmt.Sprintf("  %s\n", v))
		} else {
			b.WriteString("  N/A\n")
		}
	}

	// Connectivity Section
	b.WriteString("\nConnectivity:\n")
	b.WriteString(fmt.Sprintf("  %-24s : %s\n", "wifi", getOrDefault(sys, "wifi")))
	b.WriteString(fmt.Sprintf("  %-24s : %s\n", "bluetooth", getOrDefault(sys, "bluetooth")))

	return b.String()
}

// getOrDefault obtiene un valor o retorna placeholder
func getOrDefault(m map[string]string, key string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return "N/A"
}
