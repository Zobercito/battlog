# Plan: Migración de Logs a Formato JSON

## Objetivo

Reemplazar la salida actual de logs (formato .txt) por archivos JSON estructurados para facilitar el análisis y visualización en el dashboard.

## Estructura de Archivos

```
logs/
├── logs/
│   ├── log_2026-05-09_23-41-23.json
│   └── log_2026-05-10_15-26-30.json
├── logs_todo.json
└── archivos_procesados.txt
```

## Formato JSON Propuesto

### Estructura Principal

```json
{
  "meta": {
    "device": "/org/freedesktop/UPower/devices/battery_BAT0",
    "interval_sec": 60,
    "generated_at": "2026-05-10T16:00:00Z",
    "start_time": "2025-10-01T00:00:00Z",
    "end_time": "2026-05-10T16:00:00Z",
    "total_records": 150000,
    "version": "1.0"
  },
  "records": [ ... ]
}
```

### Formato de cada Registro

```json
{
  "timestamp": 1746904050,
  "datetime": "2026-05-10T15:27:30",
  "battery": {
    "percentage": 100,
    "state": "discharging",
    "power_w": 13.663,
    "voltage": 12.134,
    "time_empty_min": 174,
    "power_profile": "power-saver"
  },
  "health": {
    "cycles": 355,
    "capacity_current_wh": 39.547,
    "capacity_design_wh": 39.547,
    "wear_pct": 0.0
  },
  "cpu": {
    "freq_ghz": 1.7,
    "freq_max_ghz": 3.0,
    "temp_c": 38,
    "load_1min": 2.04,
    "throttling": false
  },
  "memory": {
    "used_gb": 1.6,
    "total_gb": 15,
    "pct": 11,
    "swap_mb": 0
  },
  "display": {
    "brightness_pct": 100
  },
  "gpu": {
    "nvidia_active": false,
    "mode": "integrated"
  },
  "top_processes": [
    { "name": "gnome-software", "cpu_pct": 22.9, "mem_mb": 206 },
    { "name": "dockerd", "cpu_pct": 19.0, "mem_mb": 81 }
  ],
  "connectivity": {
    "wifi": true,
    "bluetooth": true
  }
}
```

## Tareas de Implementación

### 1. Parser de Logs Existentes

- Crear script/función que parsee los archivos .txt actuales
- Generar archivos .json equivalentes
- Preservar `archivos_procesados.txt` para no reprocesar

### 2. Modificar Monitor

- Cambiar salida de `monitor.go` de texto a JSON
- Generar directamente .json en lugar de .txt

### 3. Modificar Organizer

- Cambiar de escribir a `logs_todo.txt` → `logs_todo.json`
- Adaptar la lógica de concatenación para formato JSON

### 4. Mantener Compatibilidad

- Mantener archivo `logs_todo.txt` opcional para debug
- Opcional: flag en config para elegir formato de salida

## Decisiones de Diseño

1. **Compresión**: No comprimir por ahora (JSON se comprime bien con gzip si se necesita después)

2. **logs_todo.json**: Un archivo grande con todos los registros (más simple)

3. **Top Processes**: Guardar los primeros 8 (igual que ahora)

4. **Retención**: Mantener todos los logs de sesión (igual que ahora)