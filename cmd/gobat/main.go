package main

import (
	"flag"
	"fmt"
	"os"

	"gobat/internal/config"
	"gobat/internal/monitor"
	"gobat/internal/organizer"
)

func main() {
	// Parsear argumentos
	mode := flag.String("mode", "log", "Modo: log o organize")
	flag.Parse()

	switch *mode {
	case "log", "organize":
		// válido
	default:
		fmt.Fprintf(os.Stderr, "ERROR: modo desconocido %q. Usa -mode=log o -mode=organize\n", *mode)
		os.Exit(1)
	}

	// Cargar configuración
	cfg := config.Load()

	// Ejecutar según el modo seleccionado
	switch *mode {
	case "log":
		monitor.Run(cfg)
	case "organize":
		organizer.Run(cfg)
	}
}
