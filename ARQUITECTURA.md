# Arquitectura y Guia de Desarrollo

## Vision General

GoBat esta disenado con una **arquitectura modular y limpia** siguiendo principios de:
- **Separacion de responsabilidades**
- **Facil testabilidad**
- **Bajo acoplamiento**

## Capas de la Aplicacion

```
                                    cmd/gobat/main.go
                                    (Orquestacion de modos)
                                           |
                          +-----------------+------------------+
                          |                                    |
                     monitor Run()                      organizer Run()
                   (loop de monitoreo)              (lock + delegacion)
                          |                                    |
                          +-----------+--------+---------------+
                                      |        |
                                 processor/   rotator/
                              (core compartido  (rotacion y
                               de procesamiento) compresion)
                                      |        |
                          +-----------+--------+
                          |                    |
                     system/              config/
                   (datos del sist.)    (configuracion)
                          |
                     utils/
                  (utilidades)
```

## Flujos Principales

### Flujo 1: Monitoreo (`-mode=log`)

```
main()
  +-> config.Load()
  +-> monitor.Run(cfg)
       +-> system.DetectBattery()
       +-> rotator.RotateLogs()      # Rotar logs viejos
       +-> Crea archivo de log en current/
       +-> Loop cada N segundos:
       |    +-> system.GetBatteryInfo()
       |    +-> system.GetSystemInfo()
       |    +-> Construye LogEntry JSON (via helpers)
       |    +-> Escribe a archivo + fsync
       |    +-> Cada N iteraciones: processor.ProcessSessionLogs()
       +-> (Espera Ctrl+C -> Sync + cierra JSON)
       +-> Fin
```

### Flujo 2: Organizacion (`-mode=organize`)

```
main()
  +-> config.Load()
  +-> organizer.Run(cfg)
       +-> rotator.RotateLogs()       # Rotar logs viejos primero
       +-> acquireLock()              # Lock exclusivo
       +-> processor.ProcessSessionLogs()
       |    +-> Lee control de procesados
       |    +-> Abre master_YYYY-MM.jsonl (por mes actual)
       |    +-> Adquiere flock en master (protege escritura concurrente)
       |    +-> Para cada log sin procesar en current/:
       |    |    +-> Verifica flock del monitor (salta si en uso)
       |    |    +-> Parsea JSON de sesion (tolerante a EOF)
       |    |    +-> Escribe records en maestro (JSONL)
       |    |    +-> Rollback atomico si falla escritura
       |    |    +-> Marca como procesado
       |    +-> Libera flock
       +-> Libera lock
```

## Modulos en Detalle

### `internal/config` - Configuracion

**Responsabilidad**: Centralizar configuracion

**Exports**:
- `Config` struct
- `Load()` func
- `Config.Validate()` - Validacion de coherencia

**Campos importantes**:
- `LogDir`: Directorio de logs de sesion (JSON por sesion) -> `logs/current/`
- `MasterDir`: Directorio de logs maestro por mes -> `logs/master/`
- `ArchiveDir`: Directorio de logs historicos comprimidos -> `logs/archive/`
- `ControlFile`: Archivos ya procesados
- `IntervaloSegundos`: Frecuencia de muestreo
- `DiasEnVivo`: Dias antes de comprimir logs de sesion (default: 7)
- `ComprimirAlRotar`: Comprimir logs al mover a archive (default: true)
- `RotarMaestroPorMes`: Rotar log maestro por mes (default: true)
- `RetencionDias`: Dias de retencion (0 = infinito)

**Variables de Entorno**:
- `GOBAT_LOG_DIR`: Sobrescribe el directorio raiz de logs

---

### `internal/system` - Datos del Sistema

**Responsabilidad**: Recopilar informacion del sistema

**Exports**:
- `BatteryInfo` (type = map[string]string)
- `SystemInfo` (type = map[string]string)
- `DetectBattery()` - Encuentra bateria con upower
- `GetBatteryInfo()` - Obtiene datos de bateria
- `GetSystemInfo()` - Obtiene CPU, memoria, GPU, procesos, etc.

**Estructura interna**:
```
system/
+- Public functions (GetBatteryInfo, GetSystemInfo)
+- Helper functions (private)
   +- getProcessorFrequency()
   +- getNVIDIAStatus()
   +- getMemoryUsage()
   +- getSwapUsage()
   +- getDisplayBrightness()
   +- getWiFiStatus()
   +- getBluetoothStatus()
   +- getTopProcesses()
```

---

### `internal/monitor` - Monitor de Bateria

**Responsabilidad**: Loop continuo de monitoreo y registro en formato JSON

**Exports**:
- `Run(cfg)` - Punto de entrada del modo monitor

**Flujo**:
1. Detecta bateria
2. Crea archivo JSON con cabecera
3. Loop infinito cada N segundos:
   - Recopila datos con system.Get*()
   - Serializa como JSON
   - Escribe a archivo + fsync()
4. Cada N iteraciones ejecuta processor.ProcessSessionLogs()
5. Captura Ctrl+C y cierra el JSON

---

### `internal/organizer` - Organizador de Logs

**Responsabilidad**: Punto de entrada del modo organize con lock exclusivo

**Exports**:
- `Run(cfg)` - Adquiere lock y delega en processor.ProcessSessionLogs()

**Internals**:
- `acquireLock()` - Obtiene lock exclusivo via archivo PID
- `isProcessAlive()` - Verifica si un PID esta vivo

---

### `internal/processor` - Procesador de Sesiones (Core Compartido)

**Responsabilidad**: Core de consolidacion de sesiones en log maestro JSONL.
Compartido entre monitor (ejecucion periodica) y organizer (ejecucion manual).

**Exports**:
- `ProcessSessionLogs(cfg)` - Procesa sesiones pendientes

**Internals**:
- `parseSessionJSON()` - Parsea sesion JSON (tolerante a EOF)
- `parseJSONLines()` - Extrae objetos JSON linea por linea
- `splitJSONObjects()` - Separa objetos JSON respetando strings
- `scanLine()` - Escanea lineas del archivo

**Features importantes**:
- **Formato JSONL**: Cada record es una linea JSON independiente
- **Atomicidad**: Rollback con Truncate si falla escritura
- **Idempotencia**: Verifica que ya fue procesado via archivo de control
- **Concurrencia**: flock en master file para escritura segura entre procesos
- **Tolerancia**: Parsea records completos ignorando el tail incompleto
- **splitJSONObjects** es string-aware (no se rompe con llaves dentro de valores)

---

### `internal/rotator` - Rotacion y Compresion

**Responsabilidad**: Rotar logs de sesion viejos a archive/ comprimido y rotar logs maestro por mes.

**Exports**:
- `RotateLogs(cfg)` - Punto de entrada, ejecutado al inicio de monitor y organizer
- `CurrentMasterPath(cfg)` - Ruta al archivo maestro del mes actual
- `SessionLogPathsSorted(cfg)` - Lista de archivos de sesion ordenados

**Flujo**:
1. Crea directorios current/, master/, archive/ si no existen
2. Mueve logs de sesion mas viejos que `DiasEnVivo` a `archive/YYYY-MM/`
3. Si `ComprimirAlRotar` esta activo, comprime con gzip
4. Si `RotarMaestroPorMes` esta activo, mueve masters de meses anteriores a archive/
5. Si `RetencionDias > 0`, elimina archivos basado en fecha del directorio

---

### `internal/utils` - Utilidades

**Responsabilidad**: Funciones compartidas reutilizables

**Exports**:
- `ReadIntFile(path)` - Lee entero de archivo
- `ReadIntFileToFloat(path, divisor)` - Lee y convierte a float
- `ParseSwapUsed(data)` - Parsea /proc/meminfo
- `GzipFile(src, dst)` - Comprime archivo con gzip atomicamente

---

## Patron de Dependencias

```
cmd/gobat/main.go
  +-> internal/config
  +-> internal/monitor
  |    +-> internal/config
  |    +-> internal/processor
  |    |    +-> internal/config
  |    |    +-> internal/rotator
  |    |         +-> internal/config
  |    |         +-> internal/utils
  |    +-> internal/rotator
  |    +-> internal/system
  |         +-> internal/utils
  +-> internal/organizer
       +-> internal/config
       +-> internal/processor (mismo que arriba)
       +-> internal/rotator
```

**Regla de oro**: Los modulos inferiores NO importan superiores (no hay dependencias circulares).

---

## Como Extender

### 1. Agregar un nuevo dato del sistema

**Ubicacion**: `internal/system/system.go` en `GetSystemInfo()`

```go
// En GetSystemInfo()
out["new_metric"] = getNewMetric()

// Agregar funcion helper
func getNewMetric() string {
    // Logica aqui
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

**Ubicacion**: `internal/config/config.go`

```go
IntervaloSegundos: 30,  // Cambiar aqui
```

---

### 3. Cambiar formato de logs

Los logs de sesion usan el struct `LogEntry` en `internal/monitor/monitor.go`. Los logs maestro usan JSONL en `internal/processor/processor.go`.

### 4. Agregar nuevo modo

**Ubicacion**: `cmd/gobat/main.go` y nuevo modulo en `internal/`

```go
// cmd/gobat/main.go
case "mymode":
    mymodule.Run(cfg)

// internal/mymodule/mymodule.go
package mymodule

func Run(cfg config.Config) {
    // Tu logica aqui
}
```

---

## Testabilidad

### Principios

1. **Funciones puras donde sea posible** - Facilita testing
2. **Inyeccion de dependencias** - Paso de `Config` como parametro
3. **Evitar side effects globales** - Todas las operaciones usan rutas del Config

### Escribir un test nuevo

```go
// mymodule_test.go
package mymodule

import (
    "testing"
    "os"
    "path/filepath"
)

func TestMyFeature(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()  // Directorio temporal automatico

    // Execute
    // Tu codigo aqui

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

## Performance y Optimizacion

1. **Sync de archivos**: Se usa `fsync()` despues de cada log para garantizar persistencia
2. **Compresion incremental**: Solo comprime archivos nuevos (guarda en `archivos_procesados.txt`)
3. **Lock files**: Previene multiples instancias organizando simultaneamente
4. **Lectura eficiente**: Lee `/proc/` directamente en lugar de comandos del sistema (cuando es posible)

---

## Errores Comunes y Como Evitarlos

| Error | Causa | Solucion |
|-------|-------|----------|
| "no se encontro bateria" | `upower` no devuelve salida | Instalar `upower` |
| "ya hay una instancia corriendo" | Lock file obsoleto | Borrar `.organizar.lock` |
| Tests fallan | Falta compilar | `go build ./cmd/gobat` primero |
| Permisos denegados | Acceso a `/sys` sin permisos | Normal, devuelve "unknown" |

---

## Roadmap de Refactorizacion Completado

- Separar config en modulo
- Separar monitor en modulo
- Separar organizer en modulo
- Separar processor como core compartido (elimina dependencia monitor->organizer)
- Crear modulo de sistema
- Crear modulo de utilidades
- Mover main a `cmd/gobat`
- Documentacion clara
- Tests reorganizados
- splitJSONObjects string-aware (maneja llaves dentro de strings)
- cleanupOldArchives usa fecha del directorio, no ModTime
- Config validation en Load
- GOBAT_LOG_DIR env var para directorio personalizado
- flock en masterF para escritura concurrente segura

---

**Ultima actualizacion**: 31 de mayo de 2026
