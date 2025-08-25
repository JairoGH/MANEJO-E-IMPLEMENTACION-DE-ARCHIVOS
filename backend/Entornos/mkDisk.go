package Entornos

import (
	"backend/Utils"
	"fmt"
	"os"
	"strings"
)

func MKDisk(size int, fit string, unit string, path string) string {
	var output strings.Builder

	output.WriteString("|============================================================|\n")
	output.WriteString("|======================= INICIO MKDISK ======================|\n")
	output.WriteString("|============================================================|\n")
	output.WriteString(fmt.Sprintf("  Size: %d\n  Fit: %s\n  Unit: %s\n  Path: %s\n", size, fit, unit, path))

	// Validaciones
	if fit != "bf" && fit != "wf" && fit != "ff" {
		return "  Error: Fit debe ser 'bf', 'wf' o 'ff'"
	}
	if size <= 0 {
		return "  Error: Size debe ser mayor a 0"
	}
	if unit != "k" && unit != "m" {
		return "  Error: Las unidades válidas son 'k' o 'm'"
	}

	// Crear directorios
	if err := os.MkdirAll(path[:strings.LastIndex(path, "/")], os.ModePerm); err != nil {
		return fmt.Sprintf("  Error al crear directorios: %s", err.Error())
	}

	// Crear archivo
	if err := Utils.CreateFile(path); err != nil {
		return fmt.Sprintf("  Error al crear archivo: %s", err.Error())
	}

	// Convertir tamaño a bytes
	sizeInBytes := size * 1024
	if unit == "m" {
		sizeInBytes *= 1024
	}

	// Abrir archivo
	file, err := Utils.OpenFile(path)
	if err != nil {
		return fmt.Sprintf("  Error al abrir archivo: %s", err.Error())
	}
	defer file.Close()

	// Escribir ceros
	zeroBlock := make([]byte, sizeInBytes)
	if _, err := file.Write(zeroBlock); err != nil {
		return fmt.Sprintf("  Error al escribir en el archivo: %s", err.Error())
	}

	output.WriteString("|==============================================================|\n")
	output.WriteString("|======================== FIN MKDISK ==========================|\n")
	output.WriteString("|==============================================================|\n")

	return output.String()
}
