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
     ┌────────────┐  ┌──────────────┐
     │  MONITOR   │  │ ORGANIZER    │
     │  monitor/  │  │ organizer/   │
     └────┬───────┘  └──────┬───────┘
          │                 │
          └────────┬────────┘
                   ▼
             ┌──────────┐
             │ ROTATOR  │  ← Rotación y compresión
             │ rotator/ │
             └────┬─────┘
                  │
         ┌────────┴──────────┐
         ▼                   ▼
     ┌────────────┐    ┌──────────────┐
     │   SYSTEM   │    │    CONFIG    │
     │  system/   │    │   config/    │
     └──────┬─────┘    └──────────────┘
            ▼
     ┌────────────┐
     │   UTILS    │
     │  utils/    │
     └────────────┘
```
## Flujos Principales

### Flujo 1: Monitoreo (`-mode=log`)

```
main()
  ├─> config.Load()
  ├─> monitor.Run(cfg)
  │    ├─> system.DetectBattery()
  │    ├─> rotator.RotateLogs()      # Rotar logs viejos
  │    ├─> Crea archivo de log en current/
  │    ├─> Loop cada N segundos:
  │    │    ├─> system.GetBatteryInfo()
  │    │    ├─> system.GetSystemInfo()
  │    │    ├─> Construye LogEntry JSON (vía helpers)
  │    │    └─> Escribe a archivo + fsync
  │    └─> (Espera Ctrl+C → Sync + cierra JSON)
  └─> Fin
```

### Flujo 2: Organización (`-mode=organize`)

```
main()
  ├─> config.Load()
  ├─> organizer.Run(cfg)
  │    ├─> rotator.RotateLogs()     # Rotar logs viejos primero
  │    ├─> acquireLock()
  │    ├─> Lee control de procesados
  │    ├─> Abre master_YYYY-MM.jsonl (por mes actual)
  │    ├─> Para cada log sin procesar en current/:
  │    │    ├─> Parsea JSON de sesión (tolerante a EOF)
  │    │    ├─> Escribe records en maestro (JSONL)
  │    │    └─> Marca como procesado
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
- `LogDir`: Directorio de logs de sesión (JSON por sesión) → `logs/current/`
- `MasterDir`: Directorio de logs maestro por mes → `logs/master/`
- `ArchiveDir`: Directorio de logs históricos comprimidos → `logs/archive/`
- `ControlFile`: Archivos ya procesados
- `IntervaloSegundos`: Frecuencia de muestreo
- `DiasEnVivo`: Días antes de comprimir logs de sesión (default: 7)
- `ComprimirAlRotar`: Comprimir logs al mover a archive (default: true)
- `RotarMaestroPorMes`: Rotar log maestro por mes (default: true)
- `RetencionDias`: Días de retención (0 = infinito)

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

**Responsabilidad**: Loop continuo de monitoreo y registro en formato JSON

**Exports**:
- `Run(cfg)` - Punto de entrada del modo monitor

**Internals**:
- `buildLogEntryJSON()` - Construye LogEntry como JSON
- `parseFrequency()`, `parseMemory()`, `parseProcess()`, etc.

**Flujo**:
1. Detecta batería
2. Crea archivo JSON con cabecera
3. Loop infinito cada N segundos:
   - Recopila datos con system.Get*()
   - Serializa como JSON
   - Escribe a archivo + fsync()
4. Captura Ctrl+C y cierra el JSON

---

### 📁 `internal/organizer` - Organizador de Logs

**Responsabilidad**: Consolidar logs en formato JSONL (un objeto JSON por línea)

**Exports**:
- `Run(cfg)` - Punto de entrada del modo organizador

**Internals**:
- `gzipFile()` - Comprime un archivo atomáticamente
- `parseSessionJSON()` - Parsea sesión JSON (tolerante a EOF)
- `acquireLock()` - Obtiene lock exclusivo
- `isProcessAlive()` - Verifica si un PID está vivo
- `parseJSONLines()` - Extrae objetos JSON línea por línea
- `scanLine()` - Escanea líneas del archivo

**Features importantes**:
- **Formato JSONL**: Cada record es una línea JSON independiente. Sin kill -9 ni JSON inválido.
- **Atomicidad**: Usa rollback con Truncate si falla escritura
- **Idempotencia**: Verifica qué ya fue procesado
- **Concurrencia**: Lock file previene ejecuciones paralelas
- **Recuperación**: Detecta y limpia locks huérfanos
- **Tolerancia**: Parsea records completos ignorando el tail incompleto de sesiones matadas

---

### 🔄 `internal/rotator` - Rotación y Compresión

**Responsabilidad**: Rotar logs de sesión viejos a archive/ comprimido y rotar logs maestro por mes.

**Exports**:
- `RotateLogs(cfg)` - Punto de entrada, ejecutado al inicio de monitor y organizer
- `CurrentMasterPath(cfg)` - Ruta al archivo maestro del mes actual
- `SessionLogPathsSorted(cfg)` - Lista de archivos de sesión ordenados

**Flujo**:
1. Crea directorios current/, master/, archive/ si no existen
2. Mueve logs de sesión más viejos que `DiasEnVivo` a `archive/YYYY-MM/`
3. Si `ComprimirAlRotar` está activo, comprime con gzip
4. Si `RotarMaestroPorMes` está activo, mueve masters de meses anteriores a archive/
5. Si `RetencionDias > 0`, elimina archivos más viejos que el límite

**Estructura de directorios resultante**:
```
logs/
├── current/       # Logs de sesión activos (< DiasEnVivo días)
├── master/        # Log maestro por mes (master_YYYY-MM.jsonl)
└── archive/       # Logs históricos comprimidos
    ├── 2026-04/
    └── 2026-05/
```

---

### 🔧 `internal/utils` - Utilidades

**Responsabilidad**: Funciones compartidas reutilizables

**Exports**:
- `ReadIntFile(path)` - Lee entero de archivo
- `ReadIntFileToFloat(path, divisor)` - Lee y convierte a float
- `ParseSwapUsed(data)` - Parsea /proc/meminfo

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
  │    ├─> internal/rotator
  │    │    ├─> internal/config
  │    │    └─> internal/utils
  │    └─> internal/system
  │         └─> internal/utils
  └─> internal/organizer
       ├─> internal/config
       └─> internal/rotator
            ├─> internal/config
            └─> internal/utils
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
// En buildLogEntryJSON(), usar el campo del struct
entry.NewField = value
```

---

### 2. Cambiar frecuencia de muestreo

**Ubicación**: `internal/config/config.go`

```go
IntervaloSegundos: 30,  // Cambiar aquí
```

---

### 3. Cambiar formato de logs

Los logs de sesión usan el struct `LogEntry` en `internal/monitor/monitor.go`. Los logs maestro usan JSONL en `internal/organizer/organizer.go`.

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
| Permisos denegados | Acceso a `/sys` sin permisos | Normal, devuelve "unknown" |

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
