package Estructuras

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Usuarios"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"strings"
)

// función principal del comando cat
func Cat(params map[string]string) string {
	var output strings.Builder
	output.WriteString(" ========================================================= \n")
	output.WriteString(" ===================== INICIO CAT ======================= \n")
	output.WriteString(" ========================================================= \n")

	// 1. Verificar sesión activa
	if !Usuarios.IsUserLoggedIn() {
		output.WriteString("Error: No hay una sesión activa. Use 'login' primero. \n")
		output.WriteString(" ===================== FIN CAT ======================= \n")
		return output.String()
	}

	// 2. Obtener la partición montada activa
	mountedPartition, found := getActiveMountedPartition()
	if !found {
		output.WriteString("Error: No se encontró la partición montada activa.\n")
		output.WriteString(" ===================== FIN CAT ===================== \n")
		return output.String()
	}

	// 3. Validar parámetros obligatorios
	files := getFileParams(params)
	if len(files) == 0 {
		output.WriteString("Error: Se requiere al menos un parámetro -file.\n")
		output.WriteString(" ===================== FIN CAT ===================== \n")
		return output.String()
	}

	// 4. Abrir el archivo del sistema
	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		output.WriteString(fmt.Sprintf("Error: No se pudo abrir el archivo: %v\n", err))
		output.WriteString(" ===================== FIN CAT ===================== \n")
		return output.String()
	}
	defer file.Close()

	// 5. Leer el superbloque
	var superblock Particiones.SuperBlock
	if err := Utils.ReadFile(file, &superblock, int64(mountedPartition.MountStart)); err != nil {
		output.WriteString(fmt.Sprintf("Error: No se pudo leer el Superblock: %v\n", err))
		output.WriteString(" ===================== FIN CAT ===================== \n")
		return output.String()
	}

	// 6. Procesar cada archivo
	currentUser := Usuarios.GetCurrentUser()
	for i, filePath := range files {
		output.WriteString(fmt.Sprintf("\n📄 Archivo %d: %s\n", i+1, filePath))
		output.WriteString(" ========================================== \n")

		// Usar InitSearch para encontrar el archivo
		inodeIndex, _ := Usuarios.InitSearch(filePath, file, superblock)

		if inodeIndex == -1 {
			output.WriteString("❌ Error: Archivo no encontrado\n")
			continue
		}

		// Leer el inodo
		var fileInode Particiones.Inode
		inodePos := superblock.S_inode_start + inodeIndex*int32(binary.Size(Particiones.Inode{}))
		if err := Utils.ReadFile(file, &fileInode, int64(inodePos)); err != nil {
			output.WriteString(fmt.Sprintf("❌ Error al leer inodo: %v\n", err))
			continue
		}

		// Verificar que sea un archivo regular o /users.txt (archivo especial)
		if fileInode.I_type[0] != '1' && filePath != "/users.txt" {
			output.WriteString("❌ Error: El path no corresponde a un archivo regular\n")
			continue
		}

		// Verificar permisos (excepto para /users.txt si es root)
		if filePath != "/users.txt" || currentUser.ID != 1 {
			if !hasReadPermission(fileInode, currentUser) {
				output.WriteString("❌ Error: Permisos insuficientes para leer el archivo\n")
				continue
			}
		}

		// Leer contenido usando tu función GetInodeFileData
		content, _ := Usuarios.GetInodeFileData(fileInode, file, superblock)

		// Procesar contenido para eliminar duplicados consecutivos
		uniqueContent := removeConsecutiveDuplicates(content)

		// Mostrar contenido formateado
		output.WriteString(uniqueContent)
		if !strings.HasSuffix(uniqueContent, "\n") {
			output.WriteString("\n")
		}

		output.WriteString(" ===================== \n")
	}

	output.WriteString(" ===================== FIN CAT ===================== \n")
	return output.String()
}

// Función auxiliar para eliminar duplicados consecutivos
func removeConsecutiveDuplicates(content string) string {
	if len(content) == 0 {
		return content
	}

	var result strings.Builder
	prev := content[0]
	result.WriteByte(prev)

	for i := 1; i < len(content); i++ {
		current := content[i]
		if current != prev || (i > 0 && i%12 == 0) {
			result.WriteByte(current)
			prev = current
		}
	}

	return result.String()
}

// hasReadPermission verifica permisos de lectura
func hasReadPermission(inode Particiones.Inode, user Usuarios.User) bool {
	if user.ID == 1 { // Root tiene todos los permisos
		return true
	}

	permUser := inode.I_perm[0] - '0'
	permGroup := inode.I_perm[1] - '0'
	permOther := inode.I_perm[2] - '0'

	if int32(user.ID) == inode.I_uid {
		return (permUser & 4) != 0
	}

	if int32(user.GID) == inode.I_gid {
		return (permGroup & 4) != 0
	}

	return (permOther & 4) != 0
}

// getActiveMountedPartition obtiene la partición donde está logueado el usuario
func getActiveMountedPartition() (Entornos.MountedPartition, bool) {
	currentUser := Usuarios.GetCurrentUser()
	for _, partitions := range Entornos.GetMountedPartitions() {
		for _, partition := range partitions {
			if partition.MountID == currentUser.PartitionID && partition.LoggedIn {
				return partition, true
			}
		}
	}
	return Entornos.MountedPartition{}, false
}

// getFileParams obtiene los parámetros file1, file2, etc.
func getFileParams(params map[string]string) []string {
	var files []string

	for i := 1; i <= 10; i++ {
		param := fmt.Sprintf("file%d", i)
		if path, ok := params[param]; ok {
			files = append(files, path)
		}
	}

	if len(files) == 0 {
		if path, ok := params["file"]; ok {
			files = append(files, path)
		}
	}

	return files
}
