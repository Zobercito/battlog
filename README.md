# GoBat - Monitor de Batería para Linux

Sistema modular y profesional en Go para monitorear batería en Linux, consolidar logs mensualmente y comprimir archivos automáticamente.

## 🚀 Inicio Rápido

```bash
# Compilar
go build -o gobat ./cmd/gobat

# Monitor (registra datos cada 10 segundos)
./gobat -mode=log

# Organizador (consolida y comprime logs)
./gobat -mode=organize
```

## ✨ Características

- 📊 Monitorea batería continuamente (estado, porcentaje, energía-rate, voltaje, temperatura)
- 📁 Logs de sesión en formato JSON, log maestro en JSONL (un objeto por línea)
- 🖥️ Recopila datos del sistema (CPU, GPU, memoria, procesos, conectividad)
- 🔒 Lock files previenen ejecuciones concurrentes
- 📝 Modular y profesional (sin dependencias externas)

## 📋 Requisitos

- Go 1.22+
- `upower` (para leer datos de batería)

**Instalación de dependencias:**
```bash
# Ubuntu/Debian
sudo apt install golang upower -y

# Arch
sudo pacman -S go upower
```

## 🏗️ Estructura

```
log_bateria_v3/
├── cmd/gobat/
│   └── main.go              # Punto de entrada
├── internal/                # Módulos privados
│   ├── config/              # Configuración
│   ├── monitor/             # Monitoreo de batería
│   ├── organizer/           # Organización y compresión
│   ├── rotator/             # Rotación y compresión de logs
│   ├── system/              # Datos del sistema
│   └── utils/               # Utilidades compartidas
├── tests/                   # Tests unitarios
└── logs/                    # Logs (generado)
    ├── current/             # Logs de sesión activos
    ├── master/              # Logs maestro (JSONL por mes)
    └── archive/             # Logs históricos comprimidos
```

## 📖 Documentación

| Documento | Contenido | Tiempo |
|-----------|-----------|--------|
| [QUICK_REFERENCE.md](QUICK_REFERENCE.md) | Comandos y trucos | ⏱️ 2 min |
| [ARQUITECTURA.md](ARQUITECTURA.md) | Diseño y cómo extender | ⏱️ 20 min |
| [CAMBIOS.md](CAMBIOS.md) | Estado de la refactorización y pendientes | ⏱️ 15 min |

## 💻 Uso

### Modo Monitor

Registra datos de batería cada 10 segundos:

```bash
./gobat -mode=log
```

**Output**: `logs/current/log_YYYY-MM-DD_HH-MM-SS.json`

 Presiona Ctrl+C para terminar.

### Modo Organize

Consolida logs de sesión en el log maestro (JSONL):

```bash
./gobat -mode=organize
```

**Output**:
```
Procesados 5 archivos nuevos, 0 omitidos, 0 errores.
```
**Log maestro**: `logs/master/master_YYYY-MM.jsonl` (un archivo por mes)

## 🛠️ Cambios Comunes

### Cambiar intervalo de muestreo

Edita `internal/config/config.go`:
```go
IntervaloSegundos: 10,  // Cambiar aquí (10, 30, 60 recomendado)
```

Recompila:
```bash
go build -o gobat ./cmd/gobat
```

### Agregar nuevo dato del sistema

1. Edita `internal/system/system.go` - Agrega función
2. Llámala en `GetSystemInfo()`
3. Muéstrala en `internal/monitor/monitor.go`

Ver detalles en [ARQUITECTURA.md](ARQUITECTURA.md#cómo-extender).

## 🧪 Testing

```bash
go test ./...
```

## 🐛 Troubleshooting

| Problema | Solución |
|----------|----------|
| "no se encontró batería" | `upower -e` y instala upower |
| "ya hay una instancia corriendo" | `rm logs/.organizar.lock` (Nota: los lock files ahora están en la raíz de `logs/`) |
| El binario no se crea | `go clean && go build -o gobat ./cmd/gobat` |
| Tests fallan | Primero compila, luego testa |

Ver más en [QUICK_REFERENCE.md](QUICK_REFERENCE.md#-troubleshooting-rápido).

## 📊 Módulos

| Módulo | Responsabilidad | Líneas |
|--------|-----------------|--------|
| `config/` | Configuración centralizada | 50 |
| `monitor/` | Loop de monitoreo | 250 |
| `organizer/` | Organización y consolidación | 320 |
| `rotator/` | Rotación y compresión de logs | 180 |
| `system/` | Datos del sistema | 450 |
| `utils/` | Utilidades compartidas | 110 |

## ✅ Refactorización Completada

- ✅ Código modular (5 módulos independientes)
- ✅ Sin dependencias externas
- ✅ Documentación exhaustiva
- ✅ Tests organizados
- ✅ 100% compatible con versión anterior

## 📚 Ver También

- [CAMBIOS.md](CAMBIOS.md) - Lista de bugs revisados y estado
- [ARQUITECTURA.md](ARQUITECTURA.md) - Diseño general del sistema

---

**Construido con ❤️ en Go | Última actualización: 9 de mayo de 2026**
