package Reportes

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Usuarios"
	"backend/Utils"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerarReporteFile lee el contenido exacto de un archivo del sistema de archivos simulado
// y lo guarda en un archivo de texto en la ruta especificada.
func GenerarReporteFile(pathFileLs, outputPath, id string) string {
	// --- 1. Validaciones Iniciales ---
	if !Usuarios.IsUserLoggedIn() {
		return "Error: No hay una sesión activa. Use 'login' primero."
	}
	if strings.TrimSpace(outputPath) == "" {
		return "Error: El parámetro de salida -path no puede estar vacío."
	}
	if strings.TrimSpace(id) == "" {
		return "Error: El parámetro -id no puede estar vacío."
	}
	if strings.TrimSpace(pathFileLs) == "" {
		return "Error: Debe especificar la ruta del archivo a leer con -path_file_ls."
	}

	// --- 2. Obtener Partición y Superbloque ---
	mountedPartition, found := Entornos.GetMountedPartitionByID(id)
	if !found {
		return fmt.Sprintf("Error: No se encontró la partición con ID %s montada", id)
	}

	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		return fmt.Sprintf("Error: No se pudo abrir el disco: %v", err)
	}
	defer file.Close()

	var superblock Particiones.SuperBlock
	if err := Utils.ReadFile(file, &superblock, int64(mountedPartition.MountStart)); err != nil {
		return fmt.Sprintf("Error: No se pudo leer el superbloque: %v", err)
	}

	// --- 3. Encontrar y Validar el Inodo del Archivo ---
	inodeIndex, _ := Usuarios.InitSearch(pathFileLs, file, superblock)
	if inodeIndex == -1 {
		return fmt.Sprintf("Error: Archivo '%s' no encontrado en el sistema de archivos.", pathFileLs)
	}

	var fileInode Particiones.Inode
	inodePos := superblock.S_inode_start + inodeIndex*superblock.S_inode_size
	if err := Utils.ReadFile(file, &fileInode, int64(inodePos)); err != nil {
		return fmt.Sprintf("Error al leer el inodo del archivo: %v", err)
	}

	// Verificar que sea un archivo regular (tipo '1')
	if fileInode.I_type[0] != '1' {
		return fmt.Sprintf("Error: La ruta '%s' no corresponde a un archivo.", pathFileLs)
	}

	// --- 4. Leer el Contenido del Archivo (sin limpiarlo) ---
	// La función GetInodeFileData ya se encarga de concatenar los bloques y truncar al tamaño correcto.
	content, log := Usuarios.GetInodeFileData(fileInode, file, superblock)
	fmt.Println(log) // Imprime el log de la lectura de bloques para depuración

	// --- 5. Guardar el Contenido Exacto en el Archivo de Salida ---
	// Asegurarse de que el directorio de salida exista
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Sprintf("Error al crear directorios de salida: %v", err)
	}
	// Escribir el contenido crudo al archivo
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Sprintf("Error al guardar el archivo de reporte: %v", err)
	}

	return fmt.Sprintf("Contenido del archivo '%s' guardado exitosamente en: %s", pathFileLs, outputPath)
}
