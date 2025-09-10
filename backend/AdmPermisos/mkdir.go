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
	"strings"
	"time"
)

func Mkdir(params map[string]string) string {
	var output strings.Builder
	output.WriteString(" =========================== CREAR DIRECTORIO EN EXT2 =========================== \n")

	// Validación de parámetros básicos
	path, ok := params["path"]
	if !ok {
		output.WriteString("⚠️ Error: El parámetro -path es obligatorio.\n")
		output.WriteString(" =========================== FIN CREAR DIRECTORIO =========================== \n")
		return output.String()
	}

	// Validar parámetro -p (no debe tener valor)
	if pValue, exists := params["p"]; exists && pValue != "" {
		output.WriteString("⚠️ Error: El parámetro -p no debe recibir ningún valor.\n")
		output.WriteString(" ===========================  FIN CREAR DIRECTORIO =========================== \n")
		return output.String()
	}

	// Verificar sesión activa
	if !Usuarios.IsUserLoggedIn() {
		output.WriteString("⚠️ Error: No hay una sesión activa. Use 'login' primero.\n")
		output.WriteString(" =========================== FIN CREAR DIRECTORIO =========================== \n")
		return output.String()
	}

	// Obtener usuario actual
	currentUser := Usuarios.GetCurrentUser()
	if currentUser.ID == 0 {
		output.WriteString("⚠️ Error: No se pudo obtener información del usuario actual.\n")
		output.WriteString(" =========================== FIN CREAR DIRECTORIO =========================== \n")
		return output.String()
	}

	// Obtener partición montada activa
	mountedPartition := Entornos.GetCurrentMountedPartition()
	if mountedPartition == nil {
		output.WriteString("⚠️ Error: No se encontró una partición montada activa.\n")
		output.WriteString(" =========================== FIN CREAR DIRECTORIO =========================== \n")
		return output.String()
	}

	// Abrir archivo del sistema
	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		output.WriteString(fmt.Sprintf("⚠️ Error: No se pudo abrir el archivo: %v\n", err))
		output.WriteString(" =========================== FIN CREAR DIRECTORIO =========================== \n")
		return output.String()
	}
	defer file.Close()

	// Leer superbloque
	var superblock Particiones.SuperBlock
	if err := Utils.ReadFile(file, &superblock, int64(mountedPartition.MountStart)); err != nil {
		output.WriteString(fmt.Sprintf("⚠️ Error: No se pudo leer el Superblock: %v\n", err))
		output.WriteString(" =========================== FIN CREAR DIRECTORIO =========================== \n")
		return output.String()
	}

	// Verificar si el directorio final ya existe
	exists, _ := fileExistsInExt2(path, file, superblock)
	if exists {
		output.WriteString(fmt.Sprintf("✅ El directorio '%s' ya existe.\n", path))
		output.WriteString(" =========================== FIN CREAR DIRECTORIO =========================== \n")
		return output.String()
	}

	// Verificar y crear directorios padres si es necesario
	dirPath := filepath.Dir(path)
	if dirPath != "." && dirPath != "/" {
		if _, ok := params["p"]; ok {
			// Crear directorios padres recursivamente solo si no existen
			if exists, _ := fileExistsInExt2(dirPath, file, superblock); !exists {
				if err := createDirectoriesInExt2(dirPath, file, superblock); err != nil {
					output.WriteString(fmt.Sprintf("⚠️ Error al crear directorios padres: %v\n", err))
					output.WriteString(" =========================== FIN CREAR DIRECTORIO =========================== \n")
					return output.String()
				}
			}
		} else {
			// Verificar si existe el directorio padre
			if exists, _ := fileExistsInExt2(dirPath, file, superblock); !exists {
				output.WriteString(fmt.Sprintf("⚠️ Error: La carpeta padre '%s' no existe. Use el parámetro -p para crearla.\n", dirPath))
				output.WriteString(" =========================== FIN CREAR DIRECTORIO =========================== \n")
				return output.String()
			}
		}
	}

	// Verificar permisos en directorio padre
	if !Usuarios.IsRootLoggedIn() {
		hasPerm, err := hasPermissionInExt2(dirPath, file, superblock, currentUser, "w")
		if err != nil || !hasPerm {
			output.WriteString(fmt.Sprintf("⚠️ Error: No tiene permisos de escritura en la carpeta padre '%s'.\n", dirPath))
			output.WriteString(" =========================== FIN CREAR DIRECTORIO =========================== \n")
			return output.String()
		}
	}

	// Crear el directorio en EXT2
	dirName := filepath.Base(path)
	if err := createSingleDirectory(dirPath, dirName, file, superblock, currentUser, *mountedPartition); err != nil {
		output.WriteString(fmt.Sprintf("⚠️ Error al crear directorio: %v\n", err))
		output.WriteString(" =========================== FIN CREAR DIRECTORIO =========================== \n")
		return output.String()
	}

	output.WriteString(fmt.Sprintf("✅ Directorio '%s' creado con éxito en EXT2.\n", path))
	output.WriteString(" =========================== FIN CREAR DIRECTORIO =========================== \n")
	return output.String()
}

func createSingleDirectory(parentPath string, dirName string, file *os.File, superblock Particiones.SuperBlock, user Usuarios.User, mountedPartition Entornos.MountedPartition) error {
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
		copy(newInode.I_perm[:], "777") // Root tiene todos los permisos
	} else {
		copy(newInode.I_perm[:], "775") // Permisos por defecto para directorios
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

	// Obtener inodo del padre
	parentInodeIndex, _ := Usuarios.InitSearch(parentPath, file, superblock)
	folderBlock.B_content[1].B_inodo = parentInodeIndex // Referencia al padre
	copy(folderBlock.B_content[1].B_name[:], "..")

	// 5. Escribir el bloque de directorio
	blockPos := superblock.S_block_start + blockIndex*int32(binary.Size(Particiones.FolderBlock{}))
	if err := Utils.WriteFile(file, folderBlock, int64(blockPos)); err != nil {
		return fmt.Errorf("error al escribir bloque de directorio: %v", err)
	}

	// 6. Asignar el bloque al inodo
	newInode.I_block[0] = blockIndex
	for i := 1; i < 12; i++ {
		newInode.I_block[i] = -1
	}

	// 7. Escribir el inodo
	inodePos := superblock.S_inode_start + inodeIndex*int32(binary.Size(Particiones.Inode{}))
	if err := Utils.WriteFile(file, newInode, int64(inodePos)); err != nil {
		return fmt.Errorf("error al escribir inodo: %v", err)
	}

	// 8. Marcar inodo y bloque como usados en los bitmaps
	if err := Utils.WriteFile(file, byte(1), int64(superblock.S_bm_inode_start+inodeIndex)); err != nil {
		return fmt.Errorf("error al actualizar bitmap de inodos: %v", err)
	}
	if err := Utils.WriteFile(file, byte(1), int64(superblock.S_bm_block_start+blockIndex)); err != nil {
		return fmt.Errorf("error al actualizar bitmap de bloques: %v", err)
	}

	// 9. Actualizar el directorio padre para incluir el nuevo directorio
	if err := addFileToDirectory(parentPath, dirName, inodeIndex, file, superblock); err != nil {
		return fmt.Errorf("error al actualizar directorio padre: %v", err)
	}

	// 10. Actualizar superbloque
	superblock.S_free_inodes_count--
	superblock.S_free_blocks_count--
	copy(superblock.S_mtime[:], currentTime)
	if err := Utils.WriteFile(file, superblock, int64(mountedPartition.MountStart)); err != nil {
		return fmt.Errorf("error al actualizar superbloque: %v", err)
	}

	return nil
}
