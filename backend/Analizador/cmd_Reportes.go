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

// Normaliza rutas LÓGICAS del FS EXT2 (sin cambiar mayúsculas)
func normalizeFSPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	p = filepath.Clean(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if p != "/" && strings.HasSuffix(p, "/") {
		p = strings.TrimRight(p, "/")
	}
	return p
}

func fn_rep(input string) string {
	var output strings.Builder
	output.WriteString(" ======================= GENERAR REPORTE ======================= \n")

	fs := flag.NewFlagSet("rep", flag.ExitOnError)
	name := fs.String("name", "", "Nombre del reporte a generar (mbr, disk, inode, block, bm_inode, bm_block, sb, file, ls)")
	path := fs.String("path", "", "Ruta donde se generará el reporte (SO)")
	id := fs.String("id", "", "ID de la partición")
	pathFileLs := fs.String("path_file_ls", "", "Path lógico EXT2 (para 'file' o 'ls')")

	// Parsear parámetros con la misma regex del analizador (sin lookaheads)
	matches := paramWithValue.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		flagName := match[1]
		flagValue := strings.Trim(match[2], "\"")
		switch flagName {
		case "name", "path", "id", "path_file_ls":
			_ = fs.Set(flagName, flagValue)
		default:
			output.WriteString(fmt.Sprintf("❌ Error: Flag no encontrada: %s\n", flagName))
		}
	}

	// Obligatorios
	if *name == "" || *path == "" || *id == "" {
		return "❌ Error: Los parámetros 'name', 'path' y 'id' son obligatorios.\n"
	}

	// Normalizaciones:
	*name = strings.ToLower(*name) // solo tipo de reporte, para comparar
	*path = filepath.Clean(*path)  // ruta del SO (no cambiar case)
	if *pathFileLs != "" {
		*pathFileLs = normalizeFSPath(*pathFileLs) // ruta lógica EXT2 (no cambiar case)
	}

	// Verificar partición montada
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
		return fmt.Sprintf("❌ Error: La partición con ID '%s' no está montada.\n", *id)
	}

	// Asegurar carpeta destino
	reportsDir := filepath.Dir(*path)
	if err := os.MkdirAll(reportsDir, os.ModePerm); err != nil {
		return fmt.Sprintf("❌ Error al crear la carpeta: %s\n", reportsDir)
	}

	// Ejecutar reporte
	switch *name {
	case "mbr":
		output.WriteString(Reportes.GenerarReporteMBR(diskPath, *path))
	case "disk":
		output.WriteString(Reportes.GenerarReporteDisk(diskPath, *path))
	case "file":
		output.WriteString(Reportes.GenerarReporteFile(*pathFileLs, *path, *id))
	case "sb":
		output.WriteString(Reportes.GenerarReporteSB(diskPath, *path, *id))
	case "inode":
		output.WriteString(Reportes.GenerarReporteInodo(*pathFileLs, *path, *id))
	case "tree":
		output.WriteString(Reportes.GenerarReporteArbol(diskPath, *path, *id))
	case "block":
		output.WriteString(Reportes.GenerarReporteBloques(*pathFileLs, *path, *id))
	case "ls":
		output.WriteString(Reportes.GenerarReporteLS(*pathFileLs, *path, *id))
	case "bm_inode":
		output.WriteString(Reportes.GenerarReporteBitmapInodos(*path, *id))
	case "bm_block": // <-- corregido
		output.WriteString(Reportes.GenerarReporteBitmapBloques(*path, *id))
	default:
		output.WriteString("❌ Error: Tipo de reporte no válido.\n")
	}

	output.WriteString("\n ======================= FIN GENERAR REPORTE ======================= \n")
	return output.String()
}
