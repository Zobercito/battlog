package main

import (
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

type DataPoint struct {
	Timestamp  string
	Percentage float64
	EnergyRate float64
	Voltage    float64
	CPUUsage   float64
	CPUTemp    float64
	MemoryUsed float64
	LoadAvg    float64
}

type DashboardData struct {
	Title      string
	DataPoints []DataPoint
	StartTime  string
	EndTime    string
}

var htmlTemplate = `<!DOCTYPE html>
<html lang="es">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #1a1a2e; color: #eee; padding: 20px; }
        h1 { text-align: center; margin-bottom: 10px; color: #00d9ff; }
        .subtitle { text-align: center; color: #888; margin-bottom: 30px; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(400px, 1fr)); gap: 20px; margin-bottom: 20px; }
        .chart-container { background: #16213e; border-radius: 10px; padding: 15px; }
        .chart-container h3 { color: #00d9ff; margin-bottom: 10px; font-size: 14px; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 15px; margin-bottom: 30px; }
        .stat { background: #16213e; padding: 15px; border-radius: 8px; text-align: center; }
        .stat-value { font-size: 24px; color: #00d9ff; font-weight: bold; }
        .stat-label { font-size: 12px; color: #888; margin-top: 5px; }
    </style>
</head>
<body>
    <h1>{{.Title}}</h1>
    <p class="subtitle">{{.StartTime}} - {{.EndTime}}</p>

    <div class="stats">
        <div class="stat"><div class="stat-value">{{len .DataPoints}}</div><div class="stat-label">Registros</div></div>
        <div class="stat"><div class="stat-value" id="min-batt">-</div><div class="stat-label">Batería Min</div></div>
        <div class="stat"><div class="stat-value" id="avg-power">-</div><div class="stat-label">Consumo Promedio (W)</div></div>
        <div class="stat"><div class="stat-value" id="max-temp">-</div><div class="stat-label">Temp Max (°C)</div></div>
    </div>

    <div class="grid">
        <div class="chart-container"><h3>Porcentaje de Batería</h3><canvas id="batteryChart"></canvas></div>
        <div class="chart-container"><h3>Consumo de Energía (W)</h3><canvas id="powerChart"></canvas></div>
        <div class="chart-container"><h3>Temperatura CPU (°C)</h3><canvas id="tempChart"></canvas></div>
        <div class="chart-container"><h3>Uso de Memoria (GB)</h3><canvas id="memoryChart"></canvas></div>
    </div>

    <script>
        const data = {
            labels: [{{range .DataPoints}}"{{.Timestamp}}",{{end}}],
            battery: [{{range .DataPoints}}{{.Percentage}},{{end}}],
            power: [{{range .DataPoints}}{{.EnergyRate}},{{end}}],
            temp: [{{range .DataPoints}}{{.CPUTemp}},{{end}}],
            memory: [{{range .DataPoints}}{{.MemoryUsed}},{{end}}]
        };

        const timeLabels = data.labels.map((l, i) => i % 10 === 0 ? l : '');

        new Chart(document.getElementById('batteryChart'), {
            type: 'line',
            data: { labels: timeLabels, datasets: [{ label: '%', data: data.battery, borderColor: '#00d9ff', backgroundColor: 'rgba(0,217,255,0.1)', fill: true, tension: 0.3 }] },
            options: { responsive: true, plugins: { legend: { display: false } }, scales: { y: { min: 0, max: 100 } } }
        });

        new Chart(document.getElementById('powerChart'), {
            type: 'line',
            data: { labels: timeLabels, datasets: [{ label: 'W', data: data.power, borderColor: '#ff6b6b', backgroundColor: 'rgba(255,107,107,0.1)', fill: true, tension: 0.3 }] },
            options: { responsive: true, plugins: { legend: { display: false } } }
        });

        new Chart(document.getElementById('tempChart'), {
            type: 'line',
            data: { labels: timeLabels, datasets: [{ label: '°C', data: data.temp, borderColor: '#feca57', backgroundColor: 'rgba(254,202,87,0.1)', fill: true, tension: 0.3 }] },
            options: { responsive: true, plugins: { legend: { display: false } } }
        });

        new Chart(document.getElementById('memoryChart'), {
            type: 'line',
            data: { labels: timeLabels, datasets: [{ label: 'GB', data: data.memory, borderColor: '#48dbfb', backgroundColor: 'rgba(72,219,251,0.1)', fill: true, tension: 0.3 }] },
            options: { responsive: true, plugins: { legend: { display: false } } }
        });

        document.getElementById('min-batt').textContent = Math.min(...data.battery).toFixed(0) + '%';
        document.getElementById('avg-power').textContent = (data.power.reduce((a,b) => a+b,0) / data.power.length).toFixed(1);
        document.getElementById('max-temp').textContent = Math.max(...data.temp).toFixed(0);
    </script>
</body>
</html>`

func main() {
	logDir := flag.String("dir", "logs/logs", "Directorio con archivos de log")
	output := flag.String("output", "dashboard.html", "Archivo HTML de salida")
	flag.Parse()

	files, err := filepath.Glob(filepath.Join(*logDir, "log_*.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error buscando logs: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Println("No se encontraron archivos de log")
		os.Exit(0)
	}

	sort.Strings(files)

	var allData []DataPoint
	for _, f := range files {
		data := parseLogFile(f)
		allData = append(allData, data...)
	}

	if len(allData) == 0 {
		fmt.Println("No se pudieron extraer datos de los logs")
		os.Exit(1)
	}

	dashData := DashboardData{
		Title:      "Dashboard de Batería y Sistema",
		DataPoints: allData,
		StartTime:  allData[0].Timestamp,
		EndTime:    allData[len(allData)-1].Timestamp,
	}

	tmpl, err := template.New("dashboard").Parse(htmlTemplate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parseando template: %v\n", err)
		os.Exit(1)
	}

	f, err := os.Create(*output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creando archivo: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	if err := tmpl.Execute(f, dashData); err != nil {
		fmt.Fprintf(os.Stderr, "Error generando dashboard: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Dashboard generado: %s (%d puntos de datos)\n", *output, len(allData))
}

func parseLogFile(path string) []DataPoint {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var data []DataPoint
	entry := regexp.MustCompile(`--- (\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) ---`)
	batteryPct := regexp.MustCompile(`percentage\s+:\s+(\d+)`)
	energyRate := regexp.MustCompile(`energy-rate\s+:\s+([\d.]+)\s+W`)
	voltage := regexp.MustCompile(`voltage\s+:\s+([\d.]+)\s+V`)
	cpuTemp := regexp.MustCompile(`cpu_temp\s+:\s+(\d+)`)
	memory := regexp.MustCompile(`used\s+:\s+([\d.]+)\s+GB`)
	loadAvg := regexp.MustCompile(`load_avg \(1min\)\s+:\s+([\d.]+)`)

	lines := string(content)
	for _, match := range entry.FindAllStringSubmatch(lines, -1) {
		ts := match[1]
		start := match[0]

		section := extractSection(lines, start)

		pct := extractFloat(batteryPct, section)
		rate := extractFloat(energyRate, section)
		vol := extractFloat(voltage, section)
		tmp := extractFloat(cpuTemp, section)
		mem := extractFloat(memory, section)
		load := extractFloat(loadAvg, section)

		if pct > 0 || rate > 0 {
			data = append(data, DataPoint{
				Timestamp:  ts,
				Percentage:  pct,
				EnergyRate:  rate,
				Voltage:     vol,
				CPUTemp:     tmp,
				MemoryUsed:  mem,
				LoadAvg:     load,
			})
		}
	}

	return data
}

func extractSection(content, after string) string {
	idx := findAfter(content, after)
	if idx == -1 {
		return ""
	}
	next := findAfter(content, "--- ")
	if next == -1 || next < idx {
		return content[idx:]
	}
	return content[idx:next]
}

func findAfter(s, sub string) int {
	idx := 0
	for i := 0; i < len(s)-len(sub)+1; i++ {
		if s[i:i+len(sub)] == sub {
			idx = i + len(sub)
			break
		}
	}
	if idx >= len(s) {
		return -1
	}
	return idx
}

func extractFloat(re *regexp.Regexp, s string) float64 {
	m := re.FindStringSubmatch(s)
	if len(m) > 1 {
		v, _ := strconv.ParseFloat(m[1], 64)
		return v
	}
	return 0
}