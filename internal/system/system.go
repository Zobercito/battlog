package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gobat/internal/utils"
)

// BatteryInfo contiene información de la batería
type BatteryInfo map[string]string

// SystemInfo contiene información del sistema
type SystemInfo map[string]string

// BatteryFields son los campos principales de la batería
var BatteryFields = []string{
	"state",
	"percentage",
	"time to empty",
	"time to full",
	"energy-rate",
	"energy",
	"voltage",
	"energy-full",
	"energy-full-design",
	"charge-cycles",
	"capacity",
}

// DetectBattery encuentra la ruta de la batería usando upower
func DetectBattery() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "upower", "-e").Output()
	if err != nil {
		return "", fmt.Errorf("no se pudo ejecutar upower: %w", err)
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "battery") {
			return strings.TrimSpace(line), nil
		}
	}
	return "", fmt.Errorf("no se encontró batería")
}

// GetBatteryInfo obtiene información de la batería desde upower
func GetBatteryInfo(path string) BatteryInfo {
	m := make(BatteryInfo)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "upower", "-i", path).Output()
	if err != nil {
		return m
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, field := range BatteryFields {
			if strings.HasPrefix(trimmed, field+":") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					val := strings.TrimSpace(parts[1])
					m[key] = val
				}
				break
			}
		}
	}

	enrichBatteryInfo(m)
	return m
}

// GetSystemInfo obtiene información del sistema (CPU, memoria, procesos, etc)
func GetSystemInfo() SystemInfo {
	out := make(SystemInfo)

	// CPU frequency
	out["current_frequency"] = getProcessorFrequency()

	// CPU temperature
	if t, err := utils.ReadIntFileToFloat("/sys/class/thermal/thermal_zone0/temp", 1000.0); err == nil {
		out["cpu_temp"] = fmt.Sprintf("%.0f°C", t)
	} else {
		out["cpu_temp"] = "unknown"
	}

	// Load average
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		parts := strings.Fields(string(b))
		if len(parts) >= 1 {
			out["load_avg (1min)"] = parts[0]
		} else {
			out["load_avg (1min)"] = "unknown"
		}
	} else {
		out["load_avg (1min)"] = "unknown"
	}

	// Thermal throttling
	out["thermal_throttling"] = getThermalThrottling()

	// NVIDIA GPU status
	out["nvidia_gpu"] = getNVIDIAStatus()

	// Memory usage
	out["used"] = getMemoryUsage()

	// Swap usage
	out["swap_used"] = getSwapUsage()

	// Display brightness
	out["brightness"] = getDisplayBrightness()

	// Top processes
	top := getTopProcesses()
	for i, proc := range top {
		out[fmt.Sprintf("top%d", i+1)] = proc
	}

	// WiFi status
	out["wifi"] = getWiFiStatus()

	// Bluetooth status
	out["bluetooth"] = getBluetoothStatus()

	// Power profile
	out["power_profile"] = getPowerProfile()

	return out
}

// --- FUNCIONES INTERNAS ---

func getProcessorFrequency() string {
	paths := []struct{ cur, max string }{
		{"/sys/devices/system/cpu/cpufreq/policy0/scaling_cur_freq", "/sys/devices/system/cpu/cpufreq/policy0/cpuinfo_max_freq"},
		{"/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq", "/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq"},
	}

	for _, p := range paths {
		cur, err := utils.ReadIntFile(p.cur)
		if err != nil {
			continue
		}

		max, err2 := utils.ReadIntFile(p.max)
		if err2 == nil && max > 0 {
			curGHz := float64(cur) / 1e6
			maxGHz := float64(max) / 1e6
			return fmt.Sprintf("%.1f GHz / %.1f GHz max", curGHz, maxGHz)
		}
		return fmt.Sprintf("%d kHz", cur)
	}

	return "unknown"
}

func getNVIDIAStatus() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=utilization.gpu,memory.used", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return "off (integrated only)"
	}

	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(parts) < 2 {
		return "on (unknown state)"
	}

	util := strings.TrimSpace(parts[0])
	memUsed := strings.TrimSpace(parts[1])
	if util == "0" {
		return fmt.Sprintf("on (idle, %s MB used)", memUsed)
	}
	return fmt.Sprintf("on (%s%% util, %s MB used)", util, memUsed)
}

func getMemoryUsage() string {
	mem, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return "unknown"
	}

	var memTotal, memAvailable int64
	for _, line := range strings.Split(string(mem), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				if v, err := strconv.ParseInt(f[1], 10, 64); err == nil {
					memTotal = v
				}
			}
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				if v, err := strconv.ParseInt(f[1], 10, 64); err == nil {
					memAvailable = v
				}
			}
		}
	}

	if memTotal > 0 && memAvailable >= 0 {
		usedKB := memTotal - memAvailable
		usedGB := float64(usedKB) / (1024.0 * 1024.0)
		totalGB := float64(memTotal) / (1024.0 * 1024.0)
		return fmt.Sprintf("%.1f GB / %.0f GB (%.0f%%)", usedGB, totalGB, (usedGB/totalGB)*100)
	}
	return "unknown"
}

func getSwapUsage() string {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return "unknown"
	}

	if used, ok := utils.ParseSwapUsed(string(data)); ok {
		return fmt.Sprintf("%d MB", used)
	}
	return "unknown"
}

func getDisplayBrightness() string {
	backlightDir := "/sys/class/backlight"
	entries, err := os.ReadDir(backlightDir)
	if err != nil {
		return "unknown"
	}

	for _, entry := range entries {
		if entry.IsDir() || (entry.Type()&os.ModeSymlink != 0) {
			brightnessPath := fmt.Sprintf("%s/%s/brightness", backlightDir, entry.Name())
			maxPath := fmt.Sprintf("%s/%s/max_brightness", backlightDir, entry.Name())

			brightness, err1 := utils.ReadIntFile(brightnessPath)
			max, err2 := utils.ReadIntFile(maxPath)

			if err1 == nil && err2 == nil && max > 0 {
				percentage := int((brightness * 100) / max)
				return fmt.Sprintf("%d%%", percentage)
			}
		}
	}
	return "unknown"
}

func getWiFiStatus() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nmcli", "radio", "wifi").Output()
	if err != nil {
		return "unknown"
	}

	if strings.TrimSpace(string(out)) != "enabled" {
		return "off"
	}

	out2, err2 := exec.CommandContext(ctx, "nmcli", "-t", "connection", "show", "--active").Output()
	if err2 != nil {
		return "on (disconnected)"
	}

	for _, line := range strings.Split(string(out2), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) >= 3 && strings.Contains(fields[2], "802-11") {
			return "on (connected)"
		}
	}
	return "on (disconnected)"
}

func getBluetoothStatus() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "bluetoothctl", "show").Output()
	if err != nil {
		return "unknown"
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Powered:") {
			if strings.Contains(line, "yes") {
				return "on"
			} else if strings.Contains(line, "no") {
				return "off"
			}
		}
	}
	return "unknown"
}

func getTopProcesses() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "aux", "--sort=-pcpu").Output()
	if err != nil {
		return []string{"unavailable", "unavailable", "unavailable", "unavailable", "unavailable", "unavailable", "unavailable", "unavailable"}
	}

	selfPID := strconv.Itoa(os.Getpid())

	lines := strings.Split(string(out), "\n")
	var topProcesses []string

	for i := 1; i < len(lines) && len(topProcesses) < 8; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}

		pid := fields[1]
		if pid == selfPID {
			continue
		}
		cpuPercent := fields[2]
		memRSS := fields[5]
		command := filepath.Base(fields[10])

		memKB, err := strconv.ParseInt(memRSS, 10, 64)
		if err != nil {
			memKB = 0
		}
		memMB := memKB / 1024

		processInfo := fmt.Sprintf("%d. %s : %s%% (%d MB) (PID: %s)", len(topProcesses)+1, command, cpuPercent, memMB, pid)
		topProcesses = append(topProcesses, processInfo)
	}

	for len(topProcesses) < 8 {
		topProcesses = append(topProcesses, "unavailable")
	}

	return topProcesses[:8]
}

func enrichBatteryInfo(m BatteryInfo) {
	current, currentOK := parseBatteryFloat(m["energy-full"])
	design, designOK := parseBatteryFloat(m["energy-full-design"])
	if currentOK && designOK && design > 0 {
		wearLevel := ((design - current) / design) * 100
		if wearLevel < 0 {
			wearLevel = 0
		}
		if wearLevel > 100 {
			wearLevel = 100
		}
		m["wear_level"] = fmt.Sprintf("%.1f%%", wearLevel)
	} else {
		m["wear_level"] = "unknown"
	}
}

func parseBatteryFloat(value string) (float64, bool) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0, false
	}

	parsed, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func getThermalThrottling() string {
	coolingDir := "/sys/class/thermal"
	entries, err := os.ReadDir(coolingDir)
	if err != nil {
		return "unknown"
	}

	var maxThrottleLevel int64 = 0
	foundProcessor := false

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "cooling_device") {
			typePath := filepath.Join(coolingDir, entry.Name(), "type")
			typeData, err := os.ReadFile(typePath)
			if err != nil {
				continue
			}

			if strings.Contains(strings.ToLower(string(typeData)), "processor") {
				foundProcessor = true
				statePath := filepath.Join(coolingDir, entry.Name(), "cur_state")
				state, err := utils.ReadIntFile(statePath)
				if err == nil && state > maxThrottleLevel {
					maxThrottleLevel = state
				}
			}
		}
	}

	if !foundProcessor {
		return "not supported"
	}

	if maxThrottleLevel > 0 {
		return fmt.Sprintf("active (level %d)", maxThrottleLevel)
	}

	return "inactive"
}
func getPowerProfile() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "powerprofilesctl", "get").Output()
	if err != nil {
		return "unknown"
	}

	return strings.TrimSpace(string(out))
}
