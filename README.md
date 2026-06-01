# GoBat - Monitor de Batería para Linux

Sistema modular y profesional en Go para monitorear batería en Linux, consolidar logs mensualmente y comprimir archivos automaticamente.

## Inicio Rapido

```bash
# Compilar
go build -o gobat ./cmd/gobat

# Monitor (registra datos periodicamente, default cada 60s)
./gobat -mode=log

# Organizador (consolida y comprime logs)
./gobat -mode=organize
```

## Caracteristicas

- Monitorea bateria continuamente (estado, porcentaje, energia-rate, voltaje, temperatura)
- Logs de sesion en formato JSON, log maestro en JSONL (un objeto por linea)
- Recopila datos del sistema (CPU, GPU, memoria, procesos, conectividad)
- Lock files previenen ejecuciones concurrentes
- Modular y profesional (sin dependencias externas)

## Requisitos

- Go 1.22+
- `upower` (para leer datos de bateria)

**Instalacion de dependencias:**
```bash
# Ubuntu/Debian
sudo apt install golang upower -y

# Arch
sudo pacman -S go upower
```

## Estructura

```
log_bateria_v3/
├── cmd/gobat/
│   └── main.go              # Punto de entrada
├── internal/                # Modulos privados
│   ├── config/              # Configuracion
│   ├── monitor/             # Monitoreo de bateria
│   ├── organizer/           # Organizacion (lock + delegacion)
│   ├── processor/           # Procesamiento de sesiones (core compartido)
│   ├── rotator/             # Rotacion y compresion de logs
│   ├── system/              # Datos del sistema
│   └── utils/               # Utilidades compartidas
├── tests/                   # Tests unitarios
└── logs/                    # Logs (generado)
    ├── current/             # Logs de sesion activos
    ├── master/              # Logs maestro (JSONL por mes)
    └── archive/             # Logs historicos comprimidos
```

## Documentacion

| Documento | Contenido |
|-----------|-----------|
| [QUICK_REFERENCE.md](QUICK_REFERENCE.md) | Comandos y trucos |
| [ARQUITECTURA.md](ARQUITECTURA.md) | Diseno y como extender |

## Uso

### Modo Monitor

Registra datos de bateria periodicamente:

```bash
./gobat -mode=log
```

**Output**: `logs/current/log_YYYY-MM-DD_HH-MM-SS.json`

Presiona Ctrl+C para terminar.

### Modo Organize

Consolida logs de sesion en el log maestro (JSONL):

```bash
./gobat -mode=organize
```

**Output**:
```
Procesados 5 archivos nuevos, 0 omitidos, 0 errores.
```
**Log maestro**: `logs/master/master_YYYY-MM.jsonl` (un archivo por mes)

## Cambios Comunes

### Cambiar intervalo de muestreo

Edita `internal/config/config.go`:
```go
IntervaloSegundos: 60,  // Cambiar aqui (10, 30, 60 recomendado)
```

Recompila:
```bash
go build -o gobat ./cmd/gobat
```

### Usar directorio de logs personalizado

```bash
export GOBAT_LOG_DIR=/ruta/personalizada/logs
./gobat -mode=log
```

### Agregar nuevo dato del sistema

1. Edita `internal/system/system.go` - Agrega funcion
2. Llámala en `GetSystemInfo()`
3. Muestrala en `internal/monitor/monitor.go`

Ver detalles en [ARQUITECTURA.md](ARQUITECTURA.md#como-extender).

## Testing

```bash
go test ./...
```

## Troubleshooting

| Problema | Solucion |
|----------|----------|
| "no se encontro bateria" | `upower -e` y instala upower |
| "ya hay una instancia corriendo" | `rm logs/.organizar.lock` |
| El binario no se crea | `go clean && go build -o gobat ./cmd/gobat` |
| Tests fallan | Primero compila, luego testa |

Ver mas en [QUICK_REFERENCE.md](QUICK_REFERENCE.md#troubleshooting-rapido).

## Modulos

| Modulo | Responsabilidad | Lineas |
|--------|-----------------|--------|
| `config/` | Configuracion centralizada | 55 |
| `monitor/` | Loop de monitoreo | 250 |
| `organizer/` | Organizacion + lock exclusivo | 100 |
| `processor/` | Procesamiento de sesiones (core) | 250 |
| `rotator/` | Rotacion y compresion de logs | 200 |
| `system/` | Datos del sistema | 460 |
| `utils/` | Utilidades compartidas | 110 |

## Variables de Entorno

| Variable | Descripcion |
|----------|-------------|
| `GOBAT_LOG_DIR` | Directorio raiz de logs (sobrescribe el calculo automatico) |

---

**Construido en Go | Ultima actualizacion: 31 de mayo de 2026**
