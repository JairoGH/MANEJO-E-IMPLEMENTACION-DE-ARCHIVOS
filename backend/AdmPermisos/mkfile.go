package AdmPermisos

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Usuarios"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func Mkfile(params map[string]string) string {
	var output strings.Builder
	output.WriteString(" =========================== CREAR ARCHIVO EN EXT2 =========================== \n")

	// Validación de parámetros básicos
	path, ok := params["path"]
	if !ok {
		output.WriteString("⚠️ Error: El parámetro -path es obligatorio.\n")
		output.WriteString(" =========================== FIN CREAR ARCHIVO =========================== \n")
		return output.String()
	}

	// Validar parámetro -r (no debe tener valor)
	if rValue, exists := params["r"]; exists && rValue != "" {
		output.WriteString("⚠️ Error: El parámetro -r no debe recibir ningún valor.\n")
		output.WriteString(" =========================== FIN CREAR ARCHIVO =========================== \n")
		return output.String()
	}

	// Verificar sesión activa
	if !Usuarios.IsUserLoggedIn() {
		output.WriteString("⚠️ Error: No hay una sesión activa. Use 'login' primero.\n")
		output.WriteString(" =========================== FIN CREAR ARCHIVO =========================== \n")
		return output.String()
	}

	// Obtener usuario actual
	currentUser := Usuarios.GetCurrentUser()
	if currentUser.ID == 0 {
		output.WriteString("⚠️ Error: No se pudo obtener información del usuario actual.\n")
		output.WriteString(" =========================== FIN CREAR ARCHIVO =========================== \n")
		return output.String()
	}

	// Obtener partición montada activa
	mountedPartition := Entornos.GetCurrentMountedPartition()
	if mountedPartition == nil {
		output.WriteString("⚠️ Error: No se encontró una partición montada activa.\n")
		output.WriteString(" =========================== FIN CREAR ARCHIVO =========================== \n")
		return output.String()
	}

	// Abrir archivo del sistema
	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		output.WriteString(fmt.Sprintf("⚠️ Error: No se pudo abrir el archivo: %v\n", err))
		output.WriteString(" =========================== FIN CREAR ARCHIVO =========================== \n")
		return output.String()
	}
	defer file.Close()

	// Leer superbloque
	var superblock Particiones.SuperBlock
	if err := Utils.ReadFile(file, &superblock, int64(mountedPartition.MountStart)); err != nil {
		output.WriteString(fmt.Sprintf("⚠️ Error: No se pudo leer el Superblock: %v\n", err))
		output.WriteString(" =========================== FIN CREAR ARCHIVO =========================== \n")
		return output.String()
	}

	// Verificar y crear directorios padres si es necesario
	dirPath := filepath.Dir(path)
	if dirPath != "." && dirPath != "/" {
		if _, ok := params["r"]; ok {
			// Crear directorios padres recursivamente
			if err := createDirectoriesInExt2(dirPath, file, superblock); err != nil {
				output.WriteString(fmt.Sprintf("⚠️ Error al crear directorios padres: %v\n", err))
				output.WriteString(" =========================== FIN CREAR ARCHIVO =========================== \n")
				return output.String()
			}
		} else {
			// Verificar si existe el directorio padre
			if exists, _ := fileExistsInExt2(dirPath, file, superblock); !exists {
				output.WriteString("⚠️ Error: Las carpetas padre no existen. Use el parámetro -r para crearlas.\n")
				output.WriteString(" =========================== FIN CREAR ARCHIVO =========================== \n")
				return output.String()
			}
		}
	}

	// Verificar permisos en directorio padre
	if !Usuarios.IsRootLoggedIn() {
		hasPerm, err := hasPermissionInExt2(dirPath, file, superblock, currentUser, "w")
		if err != nil || !hasPerm {
			output.WriteString(fmt.Sprintf("⚠️ Error: No tiene permisos de escritura en la carpeta padre '%s'.\n", dirPath))
			output.WriteString(" =========================== FIN CREAR ARCHIVO =========================== \n")
			return output.String()
		}
	}

	// Verificar si el archivo ya existe
	exists, inodeIndex := fileExistsInExt2(path, file, superblock)
	if exists {
		// Eliminar el archivo existente para recrearlo (sobrescribir automáticamente)
		if err := freeInodeAndBlocks(inodeIndex, file, superblock); err != nil {
			output.WriteString(fmt.Sprintf("⚠️ Error al eliminar archivo existente: %v\n", err))
			output.WriteString(" =========================== FIN CREAR ARCHIVO =========================== \n")
			return output.String()
		}
	}

	// Crear el archivo en EXT2
	if err := createFileInExt2(path, file, superblock, currentUser, params, *mountedPartition); err != nil {
		output.WriteString(fmt.Sprintf("⚠️ Error al crear archivo: %v\n", err))
		output.WriteString(" =========================== FIN CREAR ARCHIVO =========================== \n")
		return output.String()
	}

	output.WriteString(fmt.Sprintf("✅ Archivo '%s' creado con éxito en EXT2.\n", path))
	output.WriteString(" =========================== FIN CREAR ARCHIVO =========================== \n")
	return output.String()
}

// Función para crear un archivo dentro del sistema EXT2 (versión corregida)
func createFileInExt2(path string, file *os.File, superblock Particiones.SuperBlock, user Usuarios.User, params map[string]string, mountedPartition Entornos.MountedPartition) error {
	// 1. Verificar si el archivo ya existe
	exists, existingInode := fileExistsInExt2(path, file, superblock)
	if exists {
		// Limpiar completamente el archivo existente
		if err := freeInodeAndBlocks(existingInode, file, superblock); err != nil {
			return fmt.Errorf("error al eliminar archivo existente: %v", err)
		}
	}

	// 2. Obtener un NUEVO inodo libre (siempre nuevo, no reutilizar)
	inodeIndex, err := findFreeInode(superblock, file)
	if err != nil {
		return fmt.Errorf("no hay inodos libres disponibles")
	}

	// 3. Crear y configurar el nuevo inodo (INICIALIZAR TODO)
	var newInode Particiones.Inode
	currentTime := time.Now().Format("2006-01-02 15:04:05")

	// Limpiar completamente el inodo
	for i := range newInode.I_block {
		newInode.I_block[i] = -1
	}

	copy(newInode.I_atime[:], currentTime)
	copy(newInode.I_ctime[:], currentTime)
	copy(newInode.I_mtime[:], currentTime)
	newInode.I_uid = int32(user.ID)
	newInode.I_gid = int32(user.GID)
	newInode.I_type[0] = '1' // 1 = Archivo
	newInode.I_size = 0      // Inicializar a 0

	// Establecer permisos
	if Usuarios.IsRootLoggedIn() {
		copy(newInode.I_perm[:], "777")
	} else {
		copy(newInode.I_perm[:], "664")
	}

	// 4. Manejar el contenido del archivo
	var content []byte
	if contPath, ok := params["cont"]; ok {
		data, err := os.ReadFile(contPath)
		if err != nil {
			return fmt.Errorf("no se pudo leer el archivo de contenido: %v", err)
		}
		content = data
	} else if sizeStr, ok := params["size"]; ok {
		size, err := strconv.Atoi(sizeStr)
		if err != nil || size < 0 {
			return fmt.Errorf("el tamaño debe ser un número no negativo")
		}
		if size > 12*64 {
			return fmt.Errorf("el tamaño máximo permitido es %d bytes", 12*64)
		}
		content = make([]byte, size)
		for i := 0; i < size; i++ {
			content[i] = byte('0' + (i % 10))
		}
	}
	newInode.I_size = int32(len(content))

	// 5. Asignar BLOQUES NUEVOS para el contenido
	if len(content) > 0 {
		blocksNeeded := (len(content) + 63) / 64

		// Verificar que no exceda el límite
		if blocksNeeded > 12 {
			return fmt.Errorf("demasiados bloques necesarios (%d), máximo 12", blocksNeeded)
		}

		for i := 0; i < blocksNeeded; i++ {
			// Buscar un bloque completamente nuevo
			blockIndex, err := findFreeBlock(superblock, file)
			if err != nil {
				// Limpiar los bloques ya asignados si hay error
				for j := 0; j < i; j++ {
					if newInode.I_block[j] != -1 {
						Utils.WriteFile(file, byte(0), int64(superblock.S_bm_block_start+newInode.I_block[j]))
					}
				}
				return fmt.Errorf("no hay bloques libres disponibles")
			}

			// Marcar bloque como usado en el bitmap
			if err := Utils.WriteFile(file, byte(1), int64(superblock.S_bm_block_start+blockIndex)); err != nil {
				return fmt.Errorf("error al actualizar bitmap de bloques")
			}

			// Preparar el contenido del bloque
			start := i * 64
			end := start + 64
			if end > len(content) {
				end = len(content)
			}
			blockContent := content[start:end]

			// Crear y escribir el bloque
			var fileBlock Particiones.FileBlock
			copy(fileBlock.B_content[:], blockContent)

			blockPos := superblock.S_block_start + blockIndex*int32(binary.Size(Particiones.FileBlock{}))
			if err := Utils.WriteFile(file, fileBlock, int64(blockPos)); err != nil {
				return fmt.Errorf("error al escribir bloque de archivo")
			}

			// Asignar bloque al inodo
			newInode.I_block[i] = blockIndex
		}
	}

	// 6. Escribir el inodo en disco
	inodePos := superblock.S_inode_start + inodeIndex*int32(binary.Size(Particiones.Inode{}))
	if err := Utils.WriteFile(file, newInode, int64(inodePos)); err != nil {
		return fmt.Errorf("error al escribir inodo")
	}

	// 7. Marcar inodo como usado
	if err := Utils.WriteFile(file, byte(1), int64(superblock.S_bm_inode_start+inodeIndex)); err != nil {
		return fmt.Errorf("error al actualizar bitmap de inodos")
	}

	// 8. Actualizar directorio padre
	dirPath := filepath.Dir(path)
	fileName := filepath.Base(path)
	if err := addFileToDirectory(dirPath, fileName, inodeIndex, file, superblock); err != nil {
		return fmt.Errorf("error al actualizar directorio padre: %v", err)
	}

	// 9. Actualizar superbloque
	superblock.S_free_inodes_count--
	if len(content) > 0 {
		blocksNeeded := (len(content) + 63) / 64 // Calculate blocks needed
		superblock.S_free_blocks_count -= int32(blocksNeeded)
	}
	copy(superblock.S_mtime[:], currentTime)
	if err := Utils.WriteFile(file, superblock, int64(mountedPartition.MountStart)); err != nil {
		return fmt.Errorf("error al actualizar superbloque")
	}

	return nil
}

// Función auxiliar para encontrar un inodo libre
func findFreeInode(superblock Particiones.SuperBlock, file *os.File) (int32, error) {
	for i := int32(0); i < superblock.S_inodes_count; i++ {
		var bit byte
		if err := Utils.ReadFile(file, &bit, int64(superblock.S_bm_inode_start+i)); err != nil {
			return -1, err
		}
		if bit == 0 {
			return i, nil
		}
	}
	return -1, fmt.Errorf("no hay inodos libres")
}

// Función auxiliar para encontrar un bloque libre
func findFreeBlock(superblock Particiones.SuperBlock, file *os.File) (int32, error) {
	for i := int32(0); i < superblock.S_blocks_count; i++ {
		var bit byte
		if err := Utils.ReadFile(file, &bit, int64(superblock.S_bm_block_start+i)); err != nil {
			return -1, err
		}
		if bit == 0 {
			return i, nil
		}
	}
	return -1, fmt.Errorf("no hay bloques libres")
}

// Función auxiliar para verificar si un archivo/directorio existe en EXT2
func fileExistsInExt2(path string, file *os.File, superblock Particiones.SuperBlock) (bool, int32) {
	inodeIndex, _ := Usuarios.InitSearch(path, file, superblock)
	return inodeIndex != -1, inodeIndex
}

// Función auxiliar para verificar permisos en EXT2
func hasPermissionInExt2(path string, file *os.File, superblock Particiones.SuperBlock, user Usuarios.User, permission string) (bool, error) {
	inodeIndex, _ := Usuarios.InitSearch(path, file, superblock)
	if inodeIndex == -1 {
		return false, fmt.Errorf("archivo/directorio no encontrado")
	}

	var inode Particiones.Inode
	inodePos := superblock.S_inode_start + inodeIndex*int32(binary.Size(Particiones.Inode{}))
	if err := Utils.ReadFile(file, &inode, int64(inodePos)); err != nil {
		return false, err
	}

	// Verificar permisos (similar a tu implementación actual)
	permUser := inode.I_perm[0] - '0'
	permGroup := inode.I_perm[1] - '0'
	permOther := inode.I_perm[2] - '0'

	var requiredBit uint8
	switch permission {
	case "r":
		requiredBit = 4
	case "w":
		requiredBit = 2
	case "x":
		requiredBit = 1
	default:
		return false, fmt.Errorf("permiso no válido")
	}

	// Verificar si el usuario es el dueño
	if int32(user.ID) == inode.I_uid {
		return (permUser & requiredBit) != 0, nil
	}

	// Verificar si el usuario está en el grupo
	if int32(user.GID) == inode.I_gid {
		return (permGroup & requiredBit) != 0, nil
	}

	// Permisos para otros
	return (permOther & requiredBit) != 0, nil
}

func createDirectoriesInExt2(path string, file *os.File, superblock Particiones.SuperBlock) error {
	currentUser := Usuarios.GetCurrentUser()
	parts := strings.Split(strings.Trim(path, "/"), "/")
	currentPath := ""

	for _, part := range parts {
		currentPath += "/" + part
		exists, _ := fileExistsInExt2(currentPath, file, superblock)

		if !exists {
			// Crear el directorio
			err := createDirectoryInExt2(currentPath, file, superblock, currentUser)
			if err != nil {
				return fmt.Errorf("error al crear directorio '%s': %v", currentPath, err)
			}
		}
	}
	return nil
}

func createDirectoryInExt2(path string, file *os.File, superblock Particiones.SuperBlock, user Usuarios.User) error {
	// 1. Obtener un inodo libre
	inodeIndex, err := findFreeInode(superblock, file)
	if err != nil {
		return fmt.Errorf("no hay inodos libres disponibles")
	}

	// 2. Crear y configurar el nuevo inodo (para directorio)
	var newInode Particiones.Inode
	currentTime := time.Now().Format("2006-01-02 15:04:05")
	copy(newInode.I_atime[:], currentTime)
	copy(newInode.I_ctime[:], currentTime)
	copy(newInode.I_mtime[:], currentTime)
	newInode.I_uid = int32(user.ID)
	newInode.I_gid = int32(user.GID)
	newInode.I_type[0] = '0' // 0 = Directorio

	// Permisos (775 por defecto para directorios)
	if Usuarios.IsRootLoggedIn() {
		copy(newInode.I_perm[:], "777")
	} else {
		copy(newInode.I_perm[:], "775")
	}

	// 3. Obtener un bloque libre para el directorio
	blockIndex, err := findFreeBlock(superblock, file)
	if err != nil {
		return fmt.Errorf("no hay bloques libres disponibles")
	}

	// 4. Crear bloque de directorio con entradas "." y ".."
	var folderBlock Particiones.FolderBlock
	folderBlock.B_content[0].B_inodo = inodeIndex // Referencia a sí mismo
	copy(folderBlock.B_content[0].B_name[:], ".")
	folderBlock.B_content[1].B_inodo = inodeIndex // Temporalmente, se actualizará después
	copy(folderBlock.B_content[1].B_name[:], "..")

	// 5. Escribir el bloque de directorio
	blockPos := superblock.S_block_start + blockIndex*int32(binary.Size(Particiones.FolderBlock{}))
	if err := Utils.WriteFile(file, folderBlock, int64(blockPos)); err != nil {
		return fmt.Errorf("error al escribir bloque de directorio")
	}

	// 6. Asignar el bloque al inodo
	newInode.I_block[0] = blockIndex
	for i := 1; i < 12; i++ {
		newInode.I_block[i] = -1
	}

	// 7. Escribir el inodo
	inodePos := superblock.S_inode_start + inodeIndex*int32(binary.Size(Particiones.Inode{}))
	if err := Utils.WriteFile(file, newInode, int64(inodePos)); err != nil {
		return fmt.Errorf("error al escribir inodo")
	}

	// 8. Marcar inodo y bloque como usados en los bitmaps
	if err := Utils.WriteFile(file, byte(1), int64(superblock.S_bm_inode_start+inodeIndex)); err != nil {
		return fmt.Errorf("error al actualizar bitmap de inodos")
	}
	if err := Utils.WriteFile(file, byte(1), int64(superblock.S_bm_block_start+blockIndex)); err != nil {
		return fmt.Errorf("error al actualizar bitmap de bloques")
	}

	// 9. Actualizar la entrada ".." en el bloque de directorio con el inodo del padre
	dirPath := filepath.Dir(path)
	if dirPath != "." && dirPath != "/" {
		parentInodeIndex, _ := Usuarios.InitSearch(dirPath, file, superblock)
		if parentInodeIndex == -1 {
			return fmt.Errorf("no se pudo encontrar el directorio padre")
		}
		folderBlock.B_content[1].B_inodo = parentInodeIndex

		// Volver a escribir el bloque actualizado
		if err := Utils.WriteFile(file, folderBlock, int64(blockPos)); err != nil {
			return fmt.Errorf("error al actualizar bloque de directorio")
		}
	}

	// 10. Actualizar el directorio padre para incluir el nuevo directorio
	if dirPath != "." && dirPath != "/" {
		dirName := filepath.Base(path)
		if err := addFileToDirectory(dirPath, dirName, inodeIndex, file, superblock); err != nil {
			return fmt.Errorf("error al actualizar directorio padre: %v", err)
		}
	}

	return nil
}

// Función auxiliar para agregar una entrada al directorio padre
func addFileToDirectory(dirPath string, fileName string, inodeIndex int32, file *os.File, superblock Particiones.SuperBlock) error {
	// Buscar inodo del directorio padre
	parentInodeIndex, _ := Usuarios.InitSearch(dirPath, file, superblock)
	if parentInodeIndex == -1 {
		return fmt.Errorf("directorio padre no encontrado")
	}

	// Leer inodo del directorio padre
	var parentInode Particiones.Inode
	parentInodePos := superblock.S_inode_start + parentInodeIndex*int32(binary.Size(Particiones.Inode{}))
	if err := Utils.ReadFile(file, &parentInode, int64(parentInodePos)); err != nil {
		return err
	}

	// Buscar un bloque de directorio con espacio libre
	for i := 0; i < 12; i++ { // Solo bloques directos
		if parentInode.I_block[i] == -1 {
			// Necesitamos asignar un nuevo bloque de directorio
			blockIndex, err := findFreeBlock(superblock, file)
			if err != nil {
				return fmt.Errorf("no hay bloques libres para directorio")
			}

			// Crear nuevo bloque de directorio
			var newFolderBlock Particiones.FolderBlock
			newFolderBlock.B_content[0].B_inodo = parentInodeIndex // Referencia a sí mismo
			copy(newFolderBlock.B_content[0].B_name[:], ".")
			newFolderBlock.B_content[1].B_inodo = parentInodeIndex // Referencia al padre
			copy(newFolderBlock.B_content[1].B_name[:], "..")
			newFolderBlock.B_content[2].B_inodo = inodeIndex // Nueva entrada
			copy(newFolderBlock.B_content[2].B_name[:], fileName)

			// Escribir el nuevo bloque
			blockPos := superblock.S_block_start + blockIndex*int32(binary.Size(Particiones.FolderBlock{}))
			if err := Utils.WriteFile(file, newFolderBlock, int64(blockPos)); err != nil {
				return err
			}

			// Actualizar bitmap de bloques
			if err := Utils.WriteFile(file, byte(1), int64(superblock.S_bm_block_start+blockIndex)); err != nil {
				return err
			}

			// Asignar bloque al inodo padre
			parentInode.I_block[i] = blockIndex
			break
		} else {
			// Buscar espacio en un bloque existente
			var folderBlock Particiones.FolderBlock
			blockPos := superblock.S_block_start + parentInode.I_block[i]*int32(binary.Size(Particiones.FolderBlock{}))
			if err := Utils.ReadFile(file, &folderBlock, int64(blockPos)); err != nil {
				return err
			}

			for j := 0; j < 4; j++ {
				if folderBlock.B_content[j].B_inodo == -1 || string(folderBlock.B_content[j].B_name[:]) == "" {
					folderBlock.B_content[j].B_inodo = inodeIndex
					copy(folderBlock.B_content[j].B_name[:], fileName)

					// Escribir el bloque actualizado
					if err := Utils.WriteFile(file, folderBlock, int64(blockPos)); err != nil {
						return err
					}
					return nil
				}
			}
		}
	}

	// Actualizar inodo padre
	if err := Utils.WriteFile(file, parentInode, int64(parentInodePos)); err != nil {
		return err
	}

	return nil
}

// Función mejorada para liberar inodo y bloques
// Función mejorada para liberar inodo y bloques
func freeInodeAndBlocks(inodeIndex int32, file *os.File, superblock Particiones.SuperBlock) error {
	var inode Particiones.Inode
	inodePos := superblock.S_inode_start + inodeIndex*int32(binary.Size(Particiones.Inode{}))
	if err := Utils.ReadFile(file, &inode, int64(inodePos)); err != nil {
		return err
	}

	// Liberar todos los bloques primero
	for i := 0; i < 12; i++ {
		if inode.I_block[i] != -1 {
			// Limpiar el contenido del bloque
			blockPos := superblock.S_block_start + inode.I_block[i]*int32(binary.Size(Particiones.FileBlock{}))
			emptyBlock := Particiones.FileBlock{}
			if err := Utils.WriteFile(file, emptyBlock, int64(blockPos)); err != nil {
				return err
			}

			// Marcar bloque como libre en el bitmap
			if err := Utils.WriteFile(file, byte(0), int64(superblock.S_bm_block_start+inode.I_block[i])); err != nil {
				return err
			}
			inode.I_block[i] = -1
		}
	}

	// Limpiar el inodo completamente
	zeroInode := Particiones.Inode{}
	if err := Utils.WriteFile(file, zeroInode, int64(inodePos)); err != nil {
		return err
	}

	// Marcar inodo como libre
	if err := Utils.WriteFile(file, byte(0), int64(superblock.S_bm_inode_start+inodeIndex)); err != nil {
		return err
	}

	return nil
}
