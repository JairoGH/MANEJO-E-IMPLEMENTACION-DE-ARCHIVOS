package Analizador

import (
	"backend/Entornos"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// Normaliza rutas del SISTEMA OPERATIVO sin cambiar mayúsculas/minúsculas.
func normalizeOSPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	return filepath.Clean(p) // no ToLower aquí
}

func fn_mkdisk(param string) string {
	paramMap := ExtractParams(param)
	var output strings.Builder

	validParams := map[string]bool{
		"size": true,
		"fit":  true,
		"unit": true,
		"path": true,
	}
	for k := range paramMap {
		if !validParams[k] {
			return fmt.Sprintf("Error: El parámetro '%s' no es válido", k)
		}
	}

	// size (obligatorio)
	sizeStr, ok := paramMap["size"]
	if !ok {
		return "Error: El parámetro 'size' es obligatorio"
	}
	size, err := strconv.Atoi(sizeStr)
	if err != nil || size <= 0 {
		return "Error: El valor de 'size' debe ser un número entero mayor que 0"
	}

	// fit (opcional)
	fit := strings.ToLower(paramMap["fit"])
	if fit == "" {
		fit = "ff"
	} else if fit != "bf" && fit != "ff" && fit != "wf" {
		return "Error: El valor de 'fit' debe ser 'bf', 'ff' o 'wf'"
	}

	// unit (opcional)
	unit := strings.ToLower(paramMap["unit"])
	if unit == "" {
		unit = "m"
	} else if unit != "k" && unit != "m" {
		return "Error: El valor de 'unit' debe ser 'k' o 'm'"
	}

	// path (obligatorio, ruta del SO → no cambiar case)
	path := normalizeOSPath(paramMap["path"])
	if path == "" {
		return "Error: El parámetro 'path' es obligatorio"
	}

	output.WriteString(Entornos.MKDisk(size, fit, unit, path))
	return output.String()
}

func fn_rmdisk(param string) string {
	paramMap := ExtractParams(param)
	var output strings.Builder

	// path del SO → no cambiar case
	path := normalizeOSPath(paramMap["path"])
	if path == "" {
		return "Error: El parámetro 'path' es obligatorio"
	}

	output.WriteString(Entornos.RmDisk(path))
	return output.String()
}

func fn_fdisk(param string) string {
	paramMap := ExtractParams(param)
	var output strings.Builder

	// unit (opcional)
	unit := strings.ToLower(paramMap["unit"])
	if unit == "" {
		unit = "k"
	} else if unit != "b" && unit != "k" && unit != "m" {
		return "Error: La unidad debe ser 'b', 'k' o 'm'"
	}

	// fit (opcional)
	fit := strings.ToLower(paramMap["fit"])
	if fit == "" {
		fit = "wf"
	} else if fit != "bf" && fit != "ff" && fit != "wf" {
		return "Error: El ajuste debe ser 'bf', 'ff' o 'wf'"
	}

	// type (opcional)
	partType := strings.ToLower(paramMap["type"])
	if partType == "" {
		partType = "p"
	} else if partType != "p" && partType != "e" && partType != "l" {
		return "Error: El tipo de partición debe ser 'p', 'e' o 'l'"
	}

	// size (obligatorio)
	size, err := strconv.Atoi(paramMap["size"])
	if err != nil || size <= 0 {
		return "Error: El valor de 'size' debe ser un número entero mayor que 0"
	}

	// name (depende de tu implementación: si Entornos lo guarda en lower, entonces ToLower)
	name := paramMap["name"]
	if name == "" {
		return "Error: El nombre de la partición es obligatorio"
	}
	// name = strings.ToLower(name) // ← descomenta solo si tu Entornos espera lower

	// path del SO → no cambiar case
	path := normalizeOSPath(paramMap["path"])
	if path == "" {
		return "Error: El parámetro 'path' es obligatorio"
	}

	// Convertir el tamaño a bytes
	sizeInBytes := size
	switch unit {
	case "k":
		sizeInBytes *= 1024
	case "m":
		sizeInBytes *= 1024 * 1024
	}

	output.WriteString(Entornos.Fdisk(sizeInBytes, path, name, unit, partType, fit))
	return output.String()
}

func fn_mount(param string) string {
	var output strings.Builder
	paramMap := ExtractParams(param)

	// path del SO → no cambiar case
	path := normalizeOSPath(paramMap["path"])
	// name: igual que en fdisk; si tu Entornos maneja lower, entonces bájalo, si no, preserva
	name := paramMap["name"]

	if path == "" || name == "" {
		return "Error: Path y Name son obligatorios"
	}

	output.WriteString(Entornos.Mount(path, name))
	return output.String()
}

// fn_mounted procesa el comando mounted.
func fn_mounted(_ string) string {
	var output strings.Builder
	mountedPartitions := Entornos.GetMountedPartitions()

	if len(mountedPartitions) == 0 {
		return "No hay Particiones Montadas."
	}

	output.WriteString(" ==================================================================== \n")
	output.WriteString(" ======================= PARTICIONES MONTADAS ======================= \n")
	output.WriteString(" ==================================================================== \n")

	for disk, partitions := range mountedPartitions {
		output.WriteString(fmt.Sprintf("  Disco: %s\n", disk))
		for _, partition := range partitions {
			output.WriteString(fmt.Sprintf("    - ID: %s\n", partition.MountID))
		}
	}
	output.WriteString(" ==================================================================== \n")
	output.WriteString(" ======================= FIN PARTICIONES ============================ \n")
	output.WriteString(" ==================================================================== \n")
	return output.String()
}
