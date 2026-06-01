# 🚀 Quick Reference - Hoja de Trucos

Comandos y operaciones más comunes. ¡Copia y pega!

---

## 💻 Compilación y Ejecución

```bash
# Compilar
go build -o gobat ./cmd/gobat

# Monitor (registra cada 10 segundos)
./gobat -mode=log

# Organizador (consolida y comprime)
./gobat -mode=organize

# Monitor con timeout (5 minutos)
timeout 300 ./gobat -mode=log

# Monitor en background
./gobat -mode=log &

# Ver proceso
ps aux | grep gobat
```

---

## 📂 Navegación de Archivos

```bash
# Ver estructura
tree -L 2 -I 'logs_*|archivos_*' .

# Ver logs de sesión activos
ls -lh logs/current/ | tail -10
cat logs/current/log_*.json | jq . | head -50

# Ver log maestro del mes actual (JSONL)
tail -100 logs/master/master_*.jsonl

# Ver qué ya fue procesado
cat logs/archivos_procesados.txt

# Contar entradas de log
wc -l logs/master/master_*.jsonl

# Buscar en logs (por ejemplo, "charging")
grep -r "charging" logs/

# Ver últimas 50 líneas en tiempo real
tail -f logs/master/master_*.jsonl
```

---

## 🔧 Cambios Comunes

### Cambiar intervalo de muestreo

**Archivo**: `internal/config/config.go`

```go
// Opción rápida: Busca esta línea
IntervaloSegundos: 10,

// Cambios recomendados:
IntervaloSegundos: 10,   // Rápido
IntervaloSegundos: 30,   // Equilibrado
IntervaloSegundos: 60,   // Ahorro
```

Luego:
```bash
go build -o gobat ./cmd/gobat
./gobat -mode=log
```

### Cambiar días antes de comprimir

**Archivo**: `internal/config/config.go`

```go
DiasEnVivo: 7,    # Comprime logs de sesión después de 7 días

# Cambios comunes:
DiasEnVivo: 1,    # Comprime muy rápido
DiasEnVivo: 14,   # Mantiene 2 semanas sin comprimir
DiasEnVivo: 30,   # Mantiene 1 mes sin comprimir
```

### Desactivar compresión automática

```go
ComprimirAlRotar: false,   # Mueve a archive/ sin comprimir
```

---

## 🧪 Testing

```bash
# Ejecutar todos los tests
go test ./... -v

# Tests con cobertura
go test ./... -cover

# Test específico
go test ./internal/organizer -run TestGzipFileCreatesValidArchive

# Verbose output
go test -v ./... 2>&1 | head -50
```

---

## 🐛 Troubleshooting Rápido

```bash
# ❌ "no se encontró batería"
upower -e
# Si devuelve nada:
sudo apt install upower -y

# ❌ "ya hay una instancia corriendo"
rm logs/.organizar.lock
./gobat -mode=organize

# ❌ Permisos denegados en /sys
# Normal. Algunos datos devuelven "placeholder" sin permisos root

# ❌ El binario no se crea
go clean && go build -o gobat ./cmd/gobat

# ❌ Tests fallan después de cambios
go build -o gobat ./cmd/gobat  # Compila primero
go test ./... -v                # Luego testa

# ❌ Ver qué pasó
dmesg | tail -20      # Mensajes del sistema
systemctl status      # Estado de servicios
```

---

## 📊 Módulos Rápido

¿Dónde está cada cosa?

```
config/       ← Configuración
monitor/      ← Monitoreo de batería
organizer/    ← Organización de logs
system/       ← Datos del sistema
utils/        ← Funciones auxiliares
```

---

## 🎯 Workflows Típicos

### Workflow 1: Monitor Continuo 24/7

```bash
# Opción 1: Screen/tmux
screen -S gobat
./gobat -mode=log
# Ctrl+A D para desconectar

# Opción 2: systemd (si lo configuras)
systemctl enable gobat
systemctl start gobat

# Opción 3: cron cada 5 minutos (poco práctico)
*/5 * * * * /path/to/gobat -mode=organize
```

### Workflow 2: Monitor + Organizar

```bash
# Terminal 1: Monitor
./gobat -mode=log &
gobat_pid=$!

# Terminal 2: Organizar cada hora
while true; do
    sleep 3600
    ./gobat -mode=organize
done

# Ctrl+C en ambas para terminar
kill $gobat_pid
```

### Workflow 3: Análisis de Logs

```bash
# Ver resumen diario
tail -100 logs/master/master_*.jsonl | jq '.timestamp'

# Ver solo porcentaje de batería
cat logs/master/master_*.jsonl | jq '.battery.percentage'

# Promedio de consumo
cat logs/master/master_*.jsonl | jq '.battery.power_w' | \
    awk '{sum+=$1; count++} END {print "Promedio: " sum/count " W"}'

# Batería mínima del día
cat logs/master/master_*.jsonl | jq '.battery.percentage' | sort -n | head -1
```

---

## 🔍 Debugging Rápido

```bash
# ¿Qué proceso está corriendo?
ps aux | grep gobat

# ¿Cuántas líneas tiene el log maestro?
wc -l logs/master/master_*.jsonl

# ¿Últimas 5 entradas?
tail -5 logs/master/master_*.jsonl | jq .

# Ver tamaño de directorios?
du -sh logs/logs/

# ¿Archivos comprimidos?
ls -lh logs/archive/*/

# ¿Lock file activo?
ls -la logs/.organizar.lock
cat logs/.organizar.lock  # Ver PID
ps aux | grep <PID>       # Ver si proceso existe

# ¿Error al compilar?
go build -o gobat ./cmd/gobat 2>&1 | head -10
```

---

## 📈 Monitoreo de Uso de Disco

```bash
# Tamaño actual
du -sh logs/

# Crecimiento diario (aproximado)
# Cada entrada ≈ 1-2 KB
# Con intervalo de 10s: 8640 entradas/día ≈ 10-20 MB/día

# Proyección a 1 año (sin comprimir)
# 20 MB/día × 365 días = 7.3 GB

# Con compresión automática de logs > 7 días: ~4-5x de ahorro
# → ~1.5-2 GB/año en archive/ con compresión gzip
```

---

## 🎨 Personalizaciones Populares

### Cambiar formato de timestamp en logs

**Archivo**: `internal/monitor/monitor.go`

Busca:
```go
time.Now().Format("2006-01-02 15:04:05")
```

Cambia a:
```go
time.Now().Format("2006-01-02 15:04:05 MST")  # Añade zona horaria
```

### Agregar nuevo dato del sistema

**Archivo**: `internal/system/system.go`

1. Añade función que devuelve el dato:
```go
func getMyData() string {
    // Tu código
    return value
}
```

2. Llámala en `GetSystemInfo()`:
```go
out["my_data"] = getMyData()
```

3. Muéstrala en `internal/monitor/monitor.go`:
```go
// En buildLogEntryJSON(), usar el campo del struct
entry.NewField = value
```

---

## 🔗 Links Útiles Internos

```bash
# Leer documentación
cat README.md                 # Guía principal
cat ARQUITECTURA.md           # Cómo está organizado el código
cat QUICK_REFERENCE.md        # Esta guía rápida
cat CAMBIOS.md                # Historial de cambios
```

---

## 💾 Backup y Recovery

```bash
# Backup completo
cp -r logs logs.backup_$(date +%Y%m%d)

# Restaurar
cp -r logs.backup_20260509 logs

# Limpiar todo (PELIGRO)
rm -rf logs/current/* logs/master/* logs/archive/* logs/archivos_procesados.txt
```

---

## ⚙️ Variables de Entorno (futuro)

```bash
# Podrían agregarse en el futuro:
GOBAT_LOG_DIR=/custom/path
GOBAT_INTERVAL=30
GOBAT_DIAS_EN_VIVO=14
GOBAT_COMPRIMIR_AL_ROTAR=true

# Uso:
export GOBAT_INTERVAL=30
./gobat -mode=log
```

---

## 🎯 Recetas Prácticas

### Receta 1: Monitorear y Notificar

```bash
#!/bin/bash
./gobat -mode=log &
while true; do
    sleep 3600
    percentage=$(tail -1 logs/master/master_*.jsonl | jq '.battery.percentage')
    if [[ $(echo "$percentage < 20" | bc) == 1 ]]; then
        notify-send "Batería baja: $percentage"
    fi
done
```

### Receta 2: Resumen Diario

```bash
#!/bin/bash
echo "=== RESUMEN DE BATERÍA DEL DÍA ==="
echo "Porcentaje mínimo:"
cat logs/master/master_*.jsonl | jq '.battery.percentage' | sort -n | head -1
echo ""
echo "Consumo promedio:"
cat logs/master/master_*.jsonl | jq '.battery.power_w' | \
    awk '{sum+=$1; count++} END {print sum/count " W"}'
```

### Receta 3: Exportar a CSV

```bash
#!/bin/bash
cat logs/master/master_*.jsonl | jq -r '[.timestamp, .battery.percentage, .cpu.temp_c, .cpu.freq_ghz] | @csv' >> battery.csv
```

---

## 📚 Referencia Rápida de Comandos Go

```bash
# Formato de código
go fmt ./cmd/gobat
go fmt ./internal/...

# Lint
go vet ./cmd/gobat
go vet ./internal/...

# Docs locales
godoc -http=:6060
# Luego abre http://localhost:6060

# Profiling (si quieres optimizar)
go test -cpuprofile=cpu.prof ./tests/
go tool pprof cpu.prof
```

---

## 🚀 Deployment Rápido

```bash
# Build optimizado
go build -ldflags="-s -w" -o gobat ./cmd/gobat

# Ver tamaño
ls -lh gobat

# Ejecutable mucho más pequeño (sin symbols/debug)
# Original: ~5-10 MB
# Con -ldflags: ~3-5 MB
```

---

## 📞 SOS - Si Todo Está Roto

```bash
# Paso 1: Verifica que Go está instalado
go version

# Paso 2: Limpia y reconstruye
go clean
go build -o gobat ./cmd/gobat

# Paso 3: Verifica que upower existe
which upower
upower -e

# Paso 4: Borra cache problemático
rm -rf $HOME/go/pkg/mod/gobat*

# Paso 5: Vuelve a compilar
go build -o gobat ./cmd/gobat

# Si aún falla: Consulta INDICE.md para links a documentación detallada
```

---

**Guardá esta página para acceso rápido.** 🚀
