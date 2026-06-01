package monitor

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"battlog/internal/config"
	"battlog/internal/processor"
	"battlog/internal/rotator"
	"battlog/internal/system"
)

// LogEntry representa un registro individual del log en formato JSON
type LogEntry struct {
	Timestamp    int64            `json:"timestamp"`
	Datetime     string           `json:"datetime"`
	Battery      BatteryData      `json:"battery"`
	Health       HealthData       `json:"health"`
	CPU          CPUData          `json:"cpu"`
	Memory       MemoryData       `json:"memory"`
	Display      DisplayData      `json:"display"`
	GPU          GPUData          `json:"gpu"`
	TopProcesses []ProcessData    `json:"top_processes"`
	Connectivity ConnectivityData `json:"connectivity"`
}

type BatteryData struct {
	Percentage   int     `json:"percentage"`
	State        string  `json:"state"`
	PowerW       float64 `json:"power_w"`
	Voltage      float64 `json:"voltage"`
	TimeEmptyMin int     `json:"time_empty_min"`
	TimeFullMin  int     `json:"time_full_min"`
	PowerProfile string  `json:"power_profile"`
	Temperature  float64 `json:"temperature"`
}

type HealthData struct {
	Cycles          int     `json:"cycles"`
	CyclesRemaining int     `json:"cycles_remaining"`
	CapacityCurrent float64 `json:"capacity_current_wh"`
	CapacityDesign  float64 `json:"capacity_design_wh"`
	WearPct         float64 `json:"wear_pct"`
}

type CPUData struct {
	FreqGHz    float64 `json:"freq_ghz"`
	FreqMaxGHz float64 `json:"freq_max_ghz"`
	TempC      int     `json:"temp_c"`
	Load1Min   float64 `json:"load_1min"`
	Throttling bool    `json:"throttling"`
}

type MemoryData struct {
	UsedGB  float64 `json:"used_gb"`
	TotalGB float64 `json:"total_gb"`
	Pct     int     `json:"pct"`
	SwapMB  int     `json:"swap_mb"`
}

type DisplayData struct {
	BrightnessPct int `json:"brightness_pct"`
}

type GPUData struct {
	NvidiaActive bool   `json:"nvidia_active"`
	Mode         string `json:"mode"`
}

type ProcessData struct {
	Name   string  `json:"name"`
	CPUPct float64 `json:"cpu_pct"`
	MemMB  int     `json:"mem_mb"`
}

type ConnectivityData struct {
	WiFi      bool `json:"wifi"`
	Bluetooth bool `json:"bluetooth"`
}

// Run inicia el monitor de batería
func Run(cfg config.Config) {
	batPath, err := system.DetectBattery()
	if err != nil {
		log.Printf("ERROR: %v", err)
		return
	}

	// Rotar logs viejos antes de empezar a monitorear
	if err := rotator.RotateLogs(cfg); err != nil {
		log.Printf("Aviso: error en rotación de logs: %v", err)
	}

	if err := os.MkdirAll(cfg.LogDir, config.DefaultPermissionDir); err != nil {
		log.Printf("Error creando directorio de logs: %v", err)
		return
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFileName := filepath.Join(cfg.LogDir, fmt.Sprintf("log_%s.json", timestamp))

	f, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, config.DefaultPermissionFile)
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

	// Escribir cabecera JSON
	header := fmt.Sprintf("{\"meta\":{\"device\":\"%s\",\"interval_sec\":%d,\"start_time\":\"%s\"},\n\"records\":[\n",
		batPath, cfg.IntervaloSegundos, time.Now().Format(time.RFC3339))
	if _, err := f.WriteString(header); err != nil {
		log.Printf("Error escribiendo cabecera de log: %v", err)
		return
	}
	if err := f.Sync(); err != nil {
		log.Printf("Aviso: no se pudo sincronizar cabecera de log: %v", err)
	}

	// Siempre cerrar el JSON al salir (incluso por panic), así el archivo nunca queda truncado
	var recordsWritten bool
	defer func() {
		if !recordsWritten {
			// Si no hay registros, cerrar el array vacío
			if _, err := f.WriteString("\n]}\n"); err != nil {
				log.Printf("Error escribiendo cierre de log: %v", err)
			}
		}
		f.Sync()
	}()

	fmt.Fprintf(os.Stderr, "Monitoreando batería %s cada %ds...\n", batPath, cfg.IntervaloSegundos)

	// Primera entrada para no prepender coma
	firstEntry := true
	iteraciones := 0
	ticker := time.NewTicker(time.Duration(cfg.IntervaloSegundos) * time.Second)

	for {
		select {
		case <-ticker.C:
			entry := buildLogEntryJSON(batPath, cfg.MaxBatteryCycles)
			entryJSON, err := json.Marshal(entry)
			if err != nil {
				log.Printf("Error serializando entrada: %v", err)
				continue
			}
			if !firstEntry {
				if _, err := f.WriteString(",\n"); err != nil {
					log.Printf("Error escribiendo separador: %v", err)
					continue
				}
			}
			firstEntry = false
			if _, err := f.Write(entryJSON); err != nil {
				log.Printf("Error escribiendo entrada de log: %v", err)
				continue
			}
			recordsWritten = true
			if err := f.Sync(); err != nil {
				log.Printf("Aviso: no se pudo sincronizar el log: %v", err)
			}

			// Organizar automáticamente cada N iteraciones
			iteraciones++
			if cfg.OrganizarCadaIteraciones > 0 && iteraciones%cfg.OrganizarCadaIteraciones == 0 {
				processor.ProcessSessionLogs(cfg)
			}
		case <-sigCh:
			ticker.Stop()
			// Sincronizar datos pendientes antes de cerrar
			if err := f.Sync(); err != nil {
				log.Printf("Aviso: no se pudo sincronizar log antes de cerrar: %v", err)
			}
			return
		}
	}
}

// buildLogEntryJSON construye una entrada del log en formato JSON
func buildLogEntryJSON(batPath string, maxCycles int) LogEntry {
	now := time.Now()
	bat := system.GetBatteryInfo(batPath, maxCycles)
	sys := system.GetSystemInfo()
	entry := LogEntry{
		Timestamp: now.Unix(),
		Datetime:  now.Format("2006-01-02 15:04:05"),
	}

	fillBatteryData(bat, sys, &entry)
	fillHealthData(bat, &entry)
	fillCPUData(sys, &entry)
	fillMemoryData(sys, &entry)
	fillGPUData(sys, &entry)
	fillDisplayData(sys, &entry)
	fillProcessesData(sys, &entry)
	fillConnectivityData(sys, &entry)

	return entry
}

func fillBatteryData(bat, sys map[string]string, entry *LogEntry) {
	if v, ok := bat["percentage"]; ok {
		if pct, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			entry.Battery.Percentage = pct
		}
	}
	if v, ok := bat["state"]; ok {
		entry.Battery.State = v
	}
	if v, ok := bat["energy-rate"]; ok {
		if val, err := strconv.ParseFloat(strings.TrimSpace(strings.Replace(v, " W", "", 1)), 64); err == nil {
			entry.Battery.PowerW = val
		}
	}
	if v, ok := bat["voltage"]; ok {
		if val, err := strconv.ParseFloat(strings.TrimSpace(strings.Replace(v, " V", "", 1)), 64); err == nil {
			entry.Battery.Voltage = val
		}
	}
	if v, ok := bat["time to empty"]; ok && v != "" {
		entry.Battery.TimeEmptyMin = parseTimeToMinutes(v)
	}
	if v, ok := bat["time to full"]; ok && v != "" {
		entry.Battery.TimeFullMin = parseTimeToMinutes(v)
	}
	if v, ok := bat["temperature"]; ok {
		parts := strings.Fields(v)
		if len(parts) > 0 {
			if val, err := strconv.ParseFloat(parts[0], 64); err == nil {
				entry.Battery.Temperature = val
			}
		}
	}
	if sys != nil {
		if v, ok := sys["power_profile"]; ok {
			entry.Battery.PowerProfile = v
		}
	}
}

func fillHealthData(bat map[string]string, entry *LogEntry) {
	if v, ok := bat["charge-cycles"]; ok {
		if cycles, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			entry.Health.Cycles = cycles
		}
	}
	if v, ok := bat["cycles_remaining"]; ok {
		if remaining, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			entry.Health.CyclesRemaining = remaining
		}
	}
	if v, ok := bat["energy-full"]; ok {
		if val, err := strconv.ParseFloat(strings.TrimSpace(strings.Replace(v, " Wh", "", 1)), 64); err == nil {
			entry.Health.CapacityCurrent = val
		}
	}
	if v, ok := bat["energy-full-design"]; ok {
		if val, err := strconv.ParseFloat(strings.TrimSpace(strings.Replace(v, " Wh", "", 1)), 64); err == nil {
			entry.Health.CapacityDesign = val
		}
	}
	if v, ok := bat["wear_level"]; ok {
		if val, err := strconv.ParseFloat(strings.TrimSpace(strings.Replace(v, "%", "", 1)), 64); err == nil {
			entry.Health.WearPct = val
		}
	}
}

func fillCPUData(sys map[string]string, entry *LogEntry) {
	if v, ok := sys["current_frequency"]; ok {
		entry.CPU.FreqGHz, entry.CPU.FreqMaxGHz = parseFrequency(v)
	}
	if v, ok := sys["cpu_temp"]; ok {
		if temp, err := strconv.Atoi(strings.TrimSpace(strings.Replace(v, "°C", "", 1))); err == nil {
			entry.CPU.TempC = temp
		}
	}
	if v, ok := sys["load_avg (1min)"]; ok {
		if val, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			entry.CPU.Load1Min = val
		}
	}
	if v, ok := sys["thermal_throttling"]; ok {
		entry.CPU.Throttling = strings.HasPrefix(v, "active")
	}
}

func fillMemoryData(sys map[string]string, entry *LogEntry) {
	if v, ok := sys["used"]; ok {
		entry.Memory.UsedGB, entry.Memory.TotalGB = parseMemory(v)
	}
	if v, ok := sys["swap_used"]; ok {
		entry.Memory.SwapMB = parseSwap(v)
	}
	// El porcentaje se parsea directamente del string original de /proc/meminfo
	// para evitar imprecisiones por conversión a float y truncado
	if v, ok := sys["memory_pct_raw"]; ok {
		if pct, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			entry.Memory.Pct = pct
		}
	} else if entry.Memory.TotalGB > 0 {
		entry.Memory.Pct = int((entry.Memory.UsedGB / entry.Memory.TotalGB) * 100)
	}
}

func fillGPUData(sys map[string]string, entry *LogEntry) {
	if v, ok := sys["nvidia_gpu"]; ok {
		entry.GPU.NvidiaActive = !strings.Contains(v, "off")
		entry.GPU.Mode = v
	}
}

func fillDisplayData(sys map[string]string, entry *LogEntry) {
	if v, ok := sys["brightness"]; ok {
		if brightness, err := strconv.Atoi(strings.TrimSpace(strings.Replace(v, "%", "", 1))); err == nil {
			entry.Display.BrightnessPct = brightness
		}
	}
}

func fillProcessesData(sys map[string]string, entry *LogEntry) {
	for i := 1; i <= 8; i++ {
		key := fmt.Sprintf("top%d", i)
		if v, ok := sys[key]; ok {
			proc := parseProcess(v)
			if proc.Name != "" {
				entry.TopProcesses = append(entry.TopProcesses, proc)
			}
		}
	}
}

func fillConnectivityData(sys map[string]string, entry *LogEntry) {
	if v, ok := sys["wifi"]; ok {
		entry.Connectivity.WiFi = strings.HasPrefix(v, "on")
	}
	if v, ok := sys["bluetooth"]; ok {
		entry.Connectivity.Bluetooth = (v == "on")
	}
}

// parseTimeToMinutes convierte tiempo "X.X hours" a minutos
func parseTimeToMinutes(t string) int {
	t = strings.TrimSpace(t)
	t = strings.TrimSuffix(t, "hours")
	t = strings.TrimSuffix(t, "hour")
	t = strings.TrimSpace(t)
	if val, err := strconv.ParseFloat(t, 64); err == nil {
		return int(val * 60)
	}
	return 0
}

// parseFrequency extrae frecuencia actual y máxima
func parseFrequency(f string) (current, max float64) {
	parts := strings.Split(f, "/")
	if len(parts) >= 2 {
		current, _ = strconv.ParseFloat(strings.TrimSpace(strings.Replace(parts[0], "GHz", "", 1)), 64)
		max, _ = strconv.ParseFloat(strings.TrimSpace(strings.Replace(parts[1], "GHz max", "", 1)), 64)
	}
	return
}

// parseMemory extrae uso y total de memoria
func parseMemory(m string) (used, total float64) {
	re := regexp.MustCompile(`([\d.]+)\s*GB\s*/\s*([\d.]+)\s*GB`)
	matches := re.FindStringSubmatch(m)
	if len(matches) >= 3 {
		used, _ = strconv.ParseFloat(matches[1], 64)
		total, _ = strconv.ParseFloat(matches[2], 64)
	}
	return
}

// parseSwap extrae swap usado en MB
func parseSwap(s string) int {
	s = strings.TrimSpace(strings.Replace(s, "MB", "", 1))
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}
	return 0
}

// parseProcess extrae nombre, cpu y memoria de un proceso
func parseProcess(p string) ProcessData {
	re := regexp.MustCompile(`^\d+\.\s*(.+?)\s*:\s*([\d.]+)%\s*\((\d+)\s*MB\)`)
	matches := re.FindStringSubmatch(p)
	if len(matches) >= 4 {
		cpu, _ := strconv.ParseFloat(matches[2], 64)
		mem, _ := strconv.Atoi(matches[3])
		return ProcessData{Name: matches[1], CPUPct: cpu, MemMB: mem}
	}
	return ProcessData{}
}
