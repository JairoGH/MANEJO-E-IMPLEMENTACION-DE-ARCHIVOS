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

	path, ok := params["path"]
	if !ok || strings.TrimSpace(path) == "" {
		return "⚠️ Error: El parámetro -path es obligatorio.\n"
	}
	path = normalizeFSPath(path)

	if !Usuarios.IsUserLoggedIn() {
		return "⚠️ Error: No hay una sesión activa. Use 'login' primero.\n"
	}

	currentUser := Usuarios.GetCurrentUser()
	mountedPartition := Entornos.GetCurrentMountedPartition()
	if mountedPartition == nil {
		return "⚠️ Error: No se encontró una partición montada activa.\n"
	}

	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		return fmt.Sprintf("⚠️ Error: No se pudo abrir el archivo: %v\n", err)
	}
	defer file.Close()

	var superblock Particiones.SuperBlock
	if err := Utils.ReadFile(file, &superblock, int64(mountedPartition.MountStart)); err != nil {
		return fmt.Sprintf("⚠️ Error: No se pudo leer el Superblock: %v\n", err)
	}
	if superblock.S_magic != 0xEF53 {
		return "⚠️ Error: La partición no tiene un sistema de archivos EXT2 formateado.\n"
	}

	// Crear directorios padres si se especifica -r
	if _, rExists := params["r"]; rExists {
		parentPath := normalizeFSPath(filepath.Dir(path))
		if err := createDirectoriesRecursive(parentPath, file, &superblock, currentUser, *mountedPartition); err != nil {
			return fmt.Sprintf("⚠️ Error al crear directorios padres para archivo: %v\n", err)
		}
	}

	// Crear el archivo final
	parentPath := normalizeFSPath(filepath.Dir(path))
	fileName := filepath.Base(path)
	if err := createSingleFile(parentPath, fileName, file, &superblock, currentUser, *mountedPartition, params); err != nil {
		return fmt.Sprintf("⚠️ Error al crear archivo '%s': %v\n", path, err)
	}

	output.WriteString(fmt.Sprintf("✅ Archivo '%s' creado con éxito en EXT2.\n", path))
	output.WriteString(" =========================== FIN CREAR ARCHIVO =========================== \n")
	return output.String()
}

// createSingleFile crea un único archivo dentro de un directorio padre existente.
func createSingleFile(parentPath, fileName string, file *os.File, sb *Particiones.SuperBlock, user Usuarios.User, part Entornos.MountedPartition, params map[string]string) error {
	parentInodeIndex, parentInode := findInodeByPath(parentPath, file, sb)
	if parentInodeIndex == -1 {
		return fmt.Errorf("la ruta padre '%s' no existe", parentPath)
	}

	if entryExistsInDir(fileName, parentInode, file, sb) {
		// Por simplicidad del proyecto, sobrescribir no se implementa.
		// Se podría agregar lógica para eliminar el archivo antiguo aquí si se desea.
		return fmt.Errorf("el archivo '%s' ya existe en '%s'", fileName, parentPath)
	}

	// 1. Encontrar inodo libre
	newInodeIndex, err := findFreeBit(sb.S_bm_inode_start, sb.S_inodes_count, file)
	if err != nil {
		return err
	}

	// 2. Preparar contenido y calcular bloques necesarios
	var content []byte
	if contPath, ok := params["cont"]; ok {
		data, err := os.ReadFile(contPath)
		if err != nil {
			return fmt.Errorf("no se pudo leer el archivo de contenido: %v", err)
		}
		content = data
	} else if sizeStr, ok := params["size"]; ok {
		size, _ := strconv.Atoi(sizeStr)
		if size > 0 {
			content = make([]byte, size)
			for i := 0; i < size; i++ {
				content[i] = byte('0' + (i % 10))
			}
		}
	}

	blocksNeeded := (len(content) + 63) / 64
	if blocksNeeded > 12 {
		return fmt.Errorf("el contenido del archivo excede el límite de 12 bloques directos")
	}
	if sb.S_free_blocks_count < int32(blocksNeeded) {
		return fmt.Errorf("no hay suficientes bloques libres para el contenido del archivo")
	}

	// 3. Crear y configurar el inodo del archivo
	var newInode Particiones.Inode
	currentTime := time.Now().Format("02/01/2006 15:04")
	newInode.I_uid = int32(user.ID)
	newInode.I_gid = int32(user.GID)
	copy(newInode.I_ctime[:], currentTime)
	copy(newInode.I_mtime[:], currentTime)
	newInode.I_type[0] = '1' // 1 = Archivo
	copy(newInode.I_perm[:], "664")
	newInode.I_size = int32(len(content))
	for i := range newInode.I_block {
		newInode.I_block[i] = -1
	}

	// 4. Asignar y escribir bloques de datos
	for i := 0; i < blocksNeeded; i++ {
		blockIndex, err := findFreeBit(sb.S_bm_block_start, sb.S_blocks_count, file)
		if err != nil {
			return err
		}
		setBit(sb.S_bm_block_start, blockIndex, file)
		newInode.I_block[i] = blockIndex

		start := i * 64
		end := start + 64
		if end > len(content) {
			end = len(content)
		}

		var fileBlock Particiones.FileBlock
		copy(fileBlock.B_content[:], content[start:end])
		blockPos := sb.S_block_start + blockIndex*int32(binary.Size(Particiones.FileBlock{}))
		Utils.WriteFile(file, fileBlock, int64(blockPos))
	}

	// 5. Escribir el nuevo inodo en disco
	inodePos := sb.S_inode_start + newInodeIndex*int32(binary.Size(Particiones.Inode{}))
	if err := Utils.WriteFile(file, newInode, int64(inodePos)); err != nil {
		return fmt.Errorf("error al escribir el nuevo inodo de archivo: %v", err)
	}

	// 6. Agregar entrada al directorio padre
	if err := addEntryToDir(fileName, newInodeIndex, parentInode, parentInodeIndex, file, sb, &part); err != nil {
		return err
	}

	// 7. Actualizar bitmaps y superbloque
	setBit(sb.S_bm_inode_start, newInodeIndex, file)
	sb.S_free_inodes_count--
	sb.S_free_blocks_count -= int32(blocksNeeded)
	if err := Utils.WriteFile(file, sb, int64(part.MountStart)); err != nil {
		return fmt.Errorf("error al actualizar superbloque para mkfile: %v", err)
	}

	return nil
}
