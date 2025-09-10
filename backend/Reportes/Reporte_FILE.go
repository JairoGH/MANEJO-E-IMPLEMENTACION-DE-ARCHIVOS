package Reportes

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Usuarios"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerarReporteFile lee el contenido de un archivo del FS EXT2 simulado
func GenerarReporteFile(pathFileLs, path, id string) string {
	var output strings.Builder

	// ====================== Validaciones básicas ======================
	if !Usuarios.IsUserLoggedIn() {
		return "Error: No hay una sesión activa. Use 'login' primero."
	}
	if strings.TrimSpace(path) == "" {
		return "Error: El parámetro de salida 'path' no puede estar vacío."
	}
	if strings.TrimSpace(id) == "" {
		return "Error: El parámetro 'id' no puede estar vacío."
	}

	// Normalizar rutas
	path = filepath.Clean(path)
	if pathFileLs != "" {
		pathFileLs = filepath.Clean(pathFileLs)
	}

	// Si no te pasan un path del FS, puedes forzar /users.txt como default.
	// Descomenta si lo deseas:
	// if pathFileLs == "" {
	// 	pathFileLs = "/users.txt"
	// }

	// ====================== Buscar partición montada ======================
	var mountedPartition Entornos.MountedPartition
	found := false
	for _, partitions := range Entornos.GetMountedPartitions() {
		for _, partition := range partitions {
			if partition.MountID == id {
				mountedPartition = partition
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return fmt.Sprintf("Error: No se encontró la partición con ID %s montada", id)
	}

	// ====================== Abrir disco y leer superbloque ======================
	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		return fmt.Sprintf("Error: No se pudo abrir el disco: %v", err)
	}
	defer file.Close()

	var superblock Particiones.SuperBlock
	if err := Utils.ReadFile(file, &superblock, int64(mountedPartition.MountStart)); err != nil {
		return fmt.Sprintf("Error: No se pudo leer el superbloque: %v", err)
	}

	// ====================== Resolver inodo del archivo ======================
	inodeIndex, _ := Usuarios.InitSearch(pathFileLs, file, superblock)
	if inodeIndex == -1 {
		return fmt.Sprintf("Error: Archivo '%s' no encontrado", pathFileLs)
	}

	var fileInode Particiones.Inode
	inodePos := superblock.S_inode_start + inodeIndex*int32(binary.Size(Particiones.Inode{}))
	if err := Utils.ReadFile(file, &fileInode, int64(inodePos)); err != nil {
		return fmt.Sprintf("Error al leer el inodo del archivo: %v", err)
	}

	// Verificar que sea archivo regular (en tu proyecto '1' es archivo)
	if fileInode.I_type[0] != '1' {
		return "Error: La ruta especificada no es un archivo regular"
	}

	// ====================== Leer y limpiar contenido ======================
	content, _ := Usuarios.GetInodeFileData(fileInode, file, superblock)
	cleanedContent := cleanFileContent(content)

	// ====================== Guardado local ======================
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Sprintf("Error al crear directorios de salida: %v", err)
	}
	if err := os.WriteFile(path, []byte(cleanedContent), 0o644); err != nil {
		return fmt.Sprintf("Error al guardar el archivo: %v", err)
	}

	output.WriteString(fmt.Sprintf("Contenido de '%s' guardado en: %s", pathFileLs, path))
	return output.String()
}

// cleanFileContent elimina bytes no imprimibles, líneas vacías y espacios extras.
func cleanFileContent(content string) string {
	// Mantener ASCII imprimible, salto de línea y tab
	cleaned := strings.Map(func(r rune) rune {
		if (r >= 32 && r <= 126) || r == '\n' || r == '\t' {
			return r
		}
		return -1
	}, content)

	// Quitar líneas vacías y trim por línea
	lines := strings.Split(cleaned, "\n")
	var result strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result.WriteString(trimmed)
			result.WriteString("\n")
		}
	}
	return strings.TrimSpace(result.String())
}
