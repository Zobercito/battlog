# 🏗️ Arquitectura y Guía de Desarrollo

## Visión General

GoBat está diseñado con una **arquitectura modular y limpia** siguiendo principios de:
- **Separación de responsabilidades**
- **Fácil testabilidad**
- **Bajo acoplamiento**

## Capas de la Aplicación

```
┌─────────────────────────────────────┐
│      cmd/gobat/main.go              │  ← Punto de entrada
│   (Orquestación de modos)           │
└──────────────┬──────────────────────┘
               │
        ┌──────┴──────┐
        ▼              ▼
    ┌────────────┐  ┌────────────┐
    │  MONITOR   │  │ ORGANIZER  │
    │  monitor/  │  │organizer/  │
    └────┬───────┘  └────┬───────┘
         │                │
         └────────┬───────┘
                  ▼
        ┌────────────────────┐
        │   SYSTEM (datos)   │  ← Recopila datos
        │    system/         │
        └────────┬───────────┘
                 │
        ┌────────┴──────────┐
        ▼                   ▼
    ┌────────────┐    ┌──────────────┐
    │  UTILS     │    │   CONFIG     │
    │ utils/     │    │  config/     │
    └────────────┘    └──────────────┘
```
## Flujos Principales

### Flujo 1: Monitoreo (`-mode=log`)

```
main()
  ├─> config.Load()
  ├─> monitor.Run(cfg)
  │    ├─> system.DetectBattery()  # Encuentra ruta de batería
  │    ├─> Crea archivo de log
  │    ├─> Loop cada N segundos:
  │    │    ├─> system.GetBatteryInfo()
  │    │    ├─> system.GetSystemInfo()
  │    │    ├─> Construye entrada de log
  │    │    └─> Escribe a archivo
  │    └─> (Espera Ctrl+C)
  └─> Fin
```

### Flujo 2: Organización (`-mode=organize`)

```
main()
  ├─> config.Load()
  ├─> organizer.Run(cfg)
  │    ├─> organizer.acquireLock()  # Evita concurrencia
  │    ├─> Lee control de procesados
  │    ├─> Para cada log sin procesar:
  │    │    ├─> Lee contenido
  │    │    ├─> Escribe en historial mensual
  │    │    ├─> Escribe en maestro
  │    │    └─> Marca como procesado
  │    ├─> organizer.compressOldLogs()  # Comprime meses antiguos
  │    └─> Libera lock
  └─> Fin
```

## Módulos en Detalle

### 📦 `internal/config` - Configuración

**Responsabilidad**: Centralizar configuración

**Exports**:
- `Config` struct
- `Load()` func

**Uso**:
```go
cfg := config.Load()
fmt.Println(cfg.LogDir)  // "/path/to/logs/logs"
```

**Campos importantes**:
- `LogDir`: Directorio de logs de sesión
- `HistorialDir`: Directorio de logs mensuales
- `MasterLog`: Log consolidado
- `IntervaloSegundos`: Frecuencia de muestreo

---

### 🖥️ `internal/system` - Datos del Sistema

**Responsabilidad**: Recopilar información del sistema

**Exports**:
- `BatteryInfo` (type = map[string]string)
- `SystemInfo` (type = map[string]string)
- `DetectBattery()` - Encuentra batería con upower
- `GetBatteryInfo()` - Obtiene datos de batería
- `GetSystemInfo()` - Obtiene CPU, memoria, GPU, procesos, etc.

**Estructura interna**:
```
system/
├─ Public functions (GetBatteryInfo, GetSystemInfo)
├─ Helper functions (private)
│  ├─ getProcessorFrequency()
│  ├─ getNVIDIAStatus()
│  ├─ getMemoryUsage()
│  ├─ getSwapUsage()
│  ├─ getDisplayBrightness()
│  ├─ getWiFiStatus()
│  ├─ getBluetoothStatus()
│  └─ getTopProcesses()
```

**Ejecución**:
```go
sys := system.GetSystemInfo()
cpu_freq := sys["current_frequency"]  // "2.4 GHz / 4.0 GHz max"
mem := sys["used"]                    // "4.2 GB / 8 GB (53%)"
```

---

### 📊 `internal/monitor` - Monitor de Batería

**Responsabilidad**: Loop continuo de monitoreo y registro

**Exports**:
- `Run(cfg)` - Punto de entrada del modo monitor

**Internals**:
- `buildLogEntry()` - Construye una entrada formateada del log
- `getOrDefault()` - Helper para valores con fallback

**Flujo**:
1. Detecta batería
2. Crea archivo de log con timestamp
3. Escribe encabezados
4. Loop infinito cada N segundos:
   - Recopila datos con `system.Get*()`
   - Formatea como texto
   - Escribe a archivo
   - `fsync()` para garantizar persistencia
5. Captura Ctrl+C y escribe pie de archivo

---

### 📁 `internal/organizer` - Organizador de Logs

**Responsabilidad**: Consolidar y comprimir logs

**Exports**:
- `Run(cfg)` - Punto de entrada del modo organizador

**Internals**:
- `compressOldLogs()` - Comprime archivos de más de N meses
- `gzipFile()` - Comprime un archivo atomáticamente
- `acquireLock()` - Obtiene lock exclusivo
- `isProcessAlive()` - Verifica si un PID está vivo

**Features importantes**:
- **Atomicidad**: Usa archivo `.tmp` durante compresión
- **Idempotencia**: Verifica qué ya fue procesado
- **Concurrencia**: Lock file previene ejecuciones paralelas
- **Recuperación**: Detecta y limpia locks huérfanos

---

### 🔧 `internal/utils` - Utilidades

**Responsabilidad**: Funciones compartidas reutilizables

**Exports**:
- `ReadIntFile(path)` - Lee entero de archivo
- `ReadIntFileToFloat(path, divisor)` - Lee y convierte a float
- `ParseSwapUsed(data)` - Parsea /proc/meminfo
- `AppendToFile(path, line)` - Añade línea a archivo

**Uso**:
```go
freq, _ := utils.ReadIntFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq")
temp, _ := utils.ReadIntFileToFloat("/sys/class/thermal/thermal_zone0/temp", 1000.0)
```

---

## Patrón de Dependencias

```
cmd/gobat/main.go
  ├─> internal/config
  ├─> internal/monitor
  │    ├─> internal/config
  │    └─> internal/system
  │         ├─> internal/utils
  │         └─> os/exec (comandos del sistema)
  └─> internal/organizer
       ├─> internal/config
       ├─> internal/utils
       └─> compress/gzip
```

**Regla de oro**: Los módulos inferiores NO importan superiores (no hay dependencias circulares).

---

## Cómo Extender

### 1. Agregar un nuevo dato del sistema

**Ubicación**: `internal/system/system.go` en `GetSystemInfo()`

```go
// En GetSystemInfo()
out["new_metric"] = getNewMetric()

// Agregar función helper
func getNewMetric() string {
    // Lógica aquí
    return "value"
}
```

**Luego en monitor**:
```go
// En buildLogEntry(), agregar sección
b.WriteString("\nNew Section:\n")
b.WriteString(fmt.Sprintf("  %-24s : %s\n", "metric", getOrDefault(sys, "new_metric")))
```

---

### 2. Cambiar frecuencia de muestreo

**Ubicación**: `internal/config/config.go`

```go
IntervaloSegundos: 30,  // Cambiar aquí
```

---

### 3. Cambiar período de compresión

**Ubicación**: `internal/config/config.go`

```go
MesesSinComprimir: 3,  // Comprimir meses con más de 3 meses de antigüedad
```

---

### 4. Agregar nuevo modo

**Ubicación**: `cmd/gobat/main.go` y nuevo módulo en `internal/`

```go
// cmd/gobat/main.go
case "mymode":
    mymodule.Run(cfg)

// internal/mymodule/mymodule.go
package mymodule

func Run(cfg config.Config) {
    // Tu lógica aquí
}
```

---

## Testabilidad

### Principios

1. **Funciones puras donde sea posible** - Facilita testing
2. **Inyección de dependencias** - Paso de `Config` como parámetro
3. **Evitar side effects globales** - Todas las operaciones usan rutas del Config

### Escribir un test nuevo

```go
// tests/mymodule_test.go
package mymodule

import (
    "testing"
    "os"
    "path/filepath"
)

func TestMyFeature(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()  // Directorio temporal automático
    
    // Execute
    // Tu código aquí
    
    // Verify
    if err != nil {
        t.Fatalf("error: %v", err)
    }
}
```

**Ejecutar**:
```bash
go test ./...
```

---

## Performance y Optimization

1. **Sync de archivos**: Se usa `fsync()` después de cada log para garantizar persistencia
2. **Compresión incremental**: Solo comprime archivos nuevos (guarda en `archivos_procesados.txt`)
3. **Lock files**: Previene múltiples instancias organizando simultáneamente
4. **Lectura eficiente**: Lee `/proc/` directamente en lugar de comandos del sistema (cuando es posible)

---

## Errores Comunes y Cómo Evitarlos

| Error | Causa | Solución |
|-------|-------|----------|
| "no se encontró batería" | `upower` no devuelve salida | Instalar `upower` |
| "ya hay una instancia corriendo" | Lock file obsoleto | Borrar `.organizar.lock` |
| Tests fallan | Falta compilar | `go build ./cmd/gobat` primero |
| Permisos denegados | Acceso a `/sys` sin permisos | Normal, devuelve "placeholder" |
| Archivos sin comprimir | Archivo es "reciente" | Cambiar `MesesSinComprimir` |

---

## Roadmap de Refactorización Completado ✅

- ✅ Separar config en módulo
- ✅ Separar monitor en módulo
- ✅ Separar organizer en módulo
- ✅ Crear módulo de sistema
- ✅ Crear módulo de utilidades
- ✅ Mover main a `cmd/gobat`
- ✅ Documentación clara
- ✅ Tests reorganizados

---

**Última actualización**: 9 de mayo de 2026
