# GoBat - Monitor de Batería para Linux

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Linux](https://img.shields.io/badge/Linux-FCC624?style=flat&logo=linux)](https://www.linux.org/)

Sistema modular y profesional en Go para monitorear batería en Linux, consolidar logs mensualmente y comprimir archivos automáticamente.

## Características

- 📊 Monitoreo continuo de batería (estado, porcentaje, energía-rate, voltaje, temperatura)
- 📁 Logs en formato JSON y JSONL para análisis posterior
- 🖥️ Recopilación de datos del sistema (CPU, GPU, memoria, procesos, conectividad)
- 🔒 Prevención de ejecuciones concurrentes con lock files
- 📦 Composición automática de logs mensuales
- 🗜️ Compresión de logs antiguos
- ⚡ Sin dependencias externas (solo stdlib de Go)

## Requisitos

### Sistema Operativo
- **Linux** (no funciona en macOS o Windows)

### Dependencias del Sistema

| Herramienta | Propósito | Instalación |
|------------|-----------|-------------|
| `upower` | Lectura de datos de batería | `sudo apt install upower` (Ubuntu/Debian) |
| `nmcli` | Estado WiFi (opcional) | `sudo apt install network-manager` |
| `bluetoothctl` | Estado Bluetooth (opcional) | `sudo apt install bluez` |
| `nvidia-smi` | Estado GPU NVIDIA (opcional) | Drivers NVIDIA propietarios |
| `powerprofilesctl` | Perfiles de energía (opcional) | `sudo apt install power-profiles-daemon` |

### Entorno de Desarrollo
- Go 1.22 o superior

## Instalación

### Desde Código Fuente

```bash
# Clonar repositorio
git clone https://github.com/tu-usuario/log_bateria_v3.git
cd log_bateria_v3

# Compilar
go build -o gobat ./cmd/gobat

# Instalar (opcional)
sudo cp gobat /usr/local/bin/
```

### Verificar Instalación

```bash
# Verificar dependencias
upower -e  # Debe mostrar dispositivos de batería

# Ejecutar prueba
./gobat -mode=log -duration=5
```

## Uso

### Modo Monitor

```bash
# Ejecutar en primer plano
./gobat -mode=log

# Ejecutar como servicio (systemd)
./gobat -mode=log -daemon
```

### Modo Organizar Logs

```bash
# Consolida logs de sesión en log maestro
./gobat -mode=organize
```

### Variables de Entorno

| Variable | Default | Descripción |
|----------|---------|-------------|
| `GOBAT_LOG_DIR` | Cálculo automático | Directorio raíz de logs |
| `GOBAT_INTERVAL` | 60 | Intervalo de muestreo en segundos |
| `GOBAT_DIAS_EN_VIVO` | 7 | Días antes de rotar logs de sesión |
| `GOBAT_COMPRIMIR` | true | Comprimir logs al rotar |
| `GOBAT_RETENCION_DIAS` | 0 | Retención en días (0 = infinito) |
| `GOBAT_MAX_CYCLES` | 1000 | Ciclos máximos estimados de batería |

Ejemplo:
```bash
export GOBAT_INTERVAL=30
export GOBAT_DIAS_EN_VIVO=14
./gobat -mode=log
```

## Estructura de Logs

```
logs/
├── current/                    # Logs de sesión actuales
│   ├── log_2026-05-30_14-30-00.json
│   └── log_2026-05-30_15-00-00.json
├── master/                     # Log maestro consolidado
│   └── master_2026-05.jsonl
└── archive/                    # Logs comprimidos
    └── logs_2026-04.tar.gz
```

## Formato de Log

Cada línea del log maestro (JSONL) contiene:
```json
{
  "timestamp": "2026-05-30T14:30:00Z",
  "battery": {
    "status": "charging",
    "percentage": 85.2,
    "energy_rate": 45.12,
    "voltage": 12.45,
    "temperature": 32.1
  },
  "system": {
    "cpu_usage": 23.5,
    "memory_usage": 67.8,
    "load_average": [1.2, 0.8, 0.5]
  }
}
```

## Estructura del Proyecto

```
log_bateria_v3/
├── cmd/gobat/
│   └── main.go              # Punto de entrada
├── internal/                # Módulos privados
│   ├── config/              # Configuración
│   ├── monitor/             # Monitoreo de batería
│   ├── organizer/           # Organización (lock + delegación)
│   ├── processor/           # Procesamiento de sesiones (core compartido)
│   ├── rotator/             # Rotación y compresión de logs
│   ├── system/              # Datos del sistema
│   └── utils/               # Utilidades compartidas
├── tests/                   # Tests unitarios
├── logs/                    # Logs (generado)
│   ├── current/             # Logs de sesión activos
│   ├── master/              # Logs maestro (JSONL por mes)
│   └── archive/             # Logs históricos comprimidos
├── LICENSE                  # Licencia MIT
└── README.md                # Este archivo
```

## Documentación

| Documento | Contenido | Tiempo de lectura |
|-----------|-----------|-------------------|
| [QUICK_REFERENCE.md](QUICK_REFERENCE.md) | Comandos y trucos | ⏱️ 2 min |
| [ARQUITECTURA.md](ARQUITECTURA.md) | Diseño y cómo extender | ⏱️ 20 min |
| [CAMBIOS.md](CAMBIOS.md) | Estado de la refactorización y pendientes | ⏱️ 15 min |

## Testing

```bash
# Ejecutar todos los tests
go test ./...

# Ejecutar con verbose
go test -v ./...

# Ejecutar con cobertura
go test -cover ./...
```

## Troubleshooting

| Problema | Solución |
|----------|----------|
| "no se encontró batería" | Verificar: `upower -e` muestra dispositivos |
| "ya hay una instancia corriendo" | Eliminar lock: `rm logs/.organizar.lock` |
| "timeout ejecutando comando" | Verificar que `upower` esté instalado y funcionando |
| GPU NVIDIA no detectada | Instalar drivers NVIDIA propietarios |
| WiFi/Bluetooth no detectado | Instalar NetworkManager/BlueZ |
| El binario no se crea | `go clean && go build -o gobat ./cmd/gobat` |
| Tests fallan | Primero compila, luego testa |

Ver más en [QUICK_REFERENCE.md](QUICK_REFERENCE.md#-troubleshooting-rápido).

## Limitaciones Conocidas

1. **Solo Linux**: Usa `/proc/` y `/sys/` específicos de Linux
2. **Topología CPU**: Asume política de frecuencia estándar (`policy0`)
3. **Zona térmica**: Usa `thermal_zone0` (puede variar según hardware)
4. **Herramientas externas**: Requiere `upower` como dependencia mínima

## Módulos

| Módulo | Responsabilidad | Líneas |
|--------|-----------------|--------|
| `config/` | Configuración centralizada | 55 |
| `monitor/` | Loop de monitoreo | 250 |
| `organizer/` | Organización + lock exclusivo | 100 |
| `processor/` | Procesamiento de sesiones (core) | 250 |
| `rotator/` | Rotación y compresión de logs | 200 |
| `system/` | Datos del sistema | 460 |
| `utils/` | Utilidades compartidas | 110 |

## Contribuir

1. Fork el proyecto
2. Crear branch para nueva feature (`git checkout -b feature/nueva-feature`)
3. Commit cambios (`git commit -am 'Agregar nueva feature'`)
4. Push al branch (`git push origin feature/nueva-feature`)
5. Abrir Pull Request

## Licencia

Este proyecto está bajo la Licencia MIT - ver el archivo [LICENSE](LICENSE) para detalles.

---

**Construido en Go**