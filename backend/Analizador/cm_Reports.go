package Analizador

import (
	"backend/Entornos"
	"backend/Reportes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func fn_rep(input string) string {

	var output strings.Builder
	output.WriteString("|=============================================================|\n")
	output.WriteString("|======================= GENERAR REPORTE =====================|\n")
	output.WriteString("|=============================================================|\n")
	fs := flag.NewFlagSet("rep", flag.ExitOnError)
	name := fs.String("name", "", "Nombre del reporte a generar (mbr, disk, inode, block, bm_inode, bm_block, sb, file, ls)")
	path := fs.String("path", "", "Ruta donde se generará el reporte")
	id := fs.String("id", "", "ID de la partición")

	// Parsear los parámetros de entrada
	matches := paramRegex.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		flagName := match[1]
		flagValue := strings.Trim(match[2], "\"")

		switch flagName {
		case "name", "path", "id", "path_file_ls":
			fs.Set(flagName, flagValue)
		default:
			output.WriteString(fmt.Sprintf("Error: Flag no encontrada: %s\n", flagName))
		}
	}

	// Verificar los parámetros obligatorios
	if *name == "" || *path == "" || *id == "" {
		return "Error: Los parámetros 'name', 'path' y 'id' son obligatorios.\n"
	}

	// Convertir el valor de path a minúsculas
	*path = strings.ToLower(*path)

	// Verificar si el disco está montado usando DiskManagement
	mounted := false
	var diskPath string
	for _, partitions := range Entornos.GetMountedPartitions() {
		for _, partition := range partitions {
			if partition.MountID == *id {
				mounted = true
				diskPath = partition.MountPath
				break
			}
		}
	}

	if !mounted {
		return fmt.Sprintf("Error: La partición con ID '%s' no está montada.\n", *id)
	}

	// Crear la carpeta si no existe
	reportsDir := filepath.Dir(*path)
	err := os.MkdirAll(reportsDir, os.ModePerm)
	if err != nil {
		return fmt.Sprintf("Error al crear la carpeta: %s\n", reportsDir)
	}

	// Generar el reporte según el tipo de reporte solicitado
	switch *name {
	case "mbr":
		output.WriteString(Reportes.GenerarReporteMBR(diskPath, *path))
	default:
		output.WriteString("Error: Tipo de reporte no válido.\n")
	}

	output.WriteString("|===================================================================|\n")
	output.WriteString("|======================= FINAL GENERAR REPORTE =====================|\n")
	output.WriteString("|===================================================================|\n")
	return output.String()
}
