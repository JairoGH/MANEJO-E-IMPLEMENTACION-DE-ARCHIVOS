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

// =================================================================================
// MKDIR - FUNCIÓN PRINCIPAL
// =================================================================================

func Mkdir(params map[string]string) string {
	var output strings.Builder
	output.WriteString(" =========================== CREAR DIRECTORIO EN EXT2 =========================== \n")

	path, ok := params["path"]
	if !ok || strings.TrimSpace(path) == "" {
		output.WriteString("⚠️ Error: El parámetro -path es obligatorio.\n")
		return output.String() + " =========================== FIN CREAR DIRECTORIO =========================== \n"
	}
	path = normalizeFSPath(path)

	if !Usuarios.IsUserLoggedIn() {
		output.WriteString("⚠️ Error: No hay una sesión activa. Use 'login' primero.\n")
		return output.String() + " =========================== FIN CREAR DIRECTORIO =========================== \n"
	}

	currentUser := Usuarios.GetCurrentUser()
	mountedPartition := Entornos.GetCurrentMountedPartition()
	if mountedPartition == nil {
		output.WriteString("⚠️ Error: No se encontró una partición montada activa.\n")
		return output.String() + " =========================== FIN CREAR DIRECTORIO =========================== \n"
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

	// Crear recursivamente si se especifica el parámetro -p
	if _, pExists := params["p"]; pExists {
		if err := createDirectoriesRecursive(path, file, &superblock, currentUser, *mountedPartition); err != nil {
			return fmt.Sprintf("⚠️ Error al crear directorios padres: %v\n", err)
		}
	} else {
		// Crear un solo directorio
		parentPath := normalizeFSPath(filepath.Dir(path))
		dirName := filepath.Base(path)

		// Verificar que el padre exista
		parentInodeIndex, _ := findInodeByPath(parentPath, file, &superblock)
		if parentInodeIndex == -1 {
			return fmt.Sprintf("⚠️ Error: La carpeta padre '%s' no existe. Use el parámetro -p para crearla.\n", parentPath)
		}

		// Crear el directorio final
		if err := createSingleDirectory(parentPath, dirName, file, &superblock, currentUser, *mountedPartition); err != nil {
			return fmt.Sprintf("⚠️ Error al crear directorio '%s': %v\n", path, err)
		}
	}

	output.WriteString(fmt.Sprintf("✅ Directorio '%s' creado con éxito en EXT2.\n", path))
	output.WriteString(" =========================== FIN CREAR DIRECTORIO =========================== \n")
	return output.String()
}

// =================================================================================
// LÓGICA DE CREACIÓN
// =================================================================================

// createDirectoriesRecursive crea todos los directorios en una ruta que no existan.
func createDirectoriesRecursive(path string, file *os.File, sb *Particiones.SuperBlock, user Usuarios.User, part Entornos.MountedPartition) error {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	currentPath := ""
	for _, partName := range parts { // Renombrada la variable para evitar shadowing
		if partName == "" {
			continue
		}
		parentPath := normalizeFSPath(currentPath)
		currentPath += "/" + partName

		// --- CORRECCIÓN CLAVE ---
		// 1. Obtener el inodo del padre.
		_, parentInode := findInodeByPath(parentPath, file, sb)
		// 2. Pasar ese inodo a entryExistsInDir.
		if !entryExistsInDir(partName, parentInode, file, sb) {
			// 3. Pasar la partición correcta (part) a createSingleDirectory.
			if err := createSingleDirectory(parentPath, partName, file, sb, user, part); err != nil {
				return err
			}
		}
	}
	return nil
}

// createSingleDirectory crea un único directorio dentro de un padre que ya existe.
func createSingleDirectory(parentPath string, dirName string, file *os.File, sb *Particiones.SuperBlock, user Usuarios.User, part Entornos.MountedPartition) error {
	parentInodeIndex, parentInode := findInodeByPath(parentPath, file, sb)
	if parentInodeIndex == -1 {
		return fmt.Errorf("la ruta padre '%s' no existe", parentPath)
	}

	if entryExistsInDir(dirName, parentInode, file, sb) {
		// En lugar de un error, simplemente retornamos nil si ya existe,
		// lo que permite que la creación recursiva continúe sin problemas.
		return nil
	}

	newInodeIndex, err := findFreeBit(sb.S_bm_inode_start, sb.S_inodes_count, file)
	if err != nil {
		return err
	}
	newBlockIndex, err := findFreeBit(sb.S_bm_block_start, sb.S_blocks_count, file)
	if err != nil {
		return err
	}

	// Crear y escribir el nuevo bloque de carpeta
	var newDirBlock Particiones.FolderBlock
	for i := range newDirBlock.B_content {
		newDirBlock.B_content[i].B_inodo = -1
	}
	copy(newDirBlock.B_content[0].B_name[:], ".")
	newDirBlock.B_content[0].B_inodo = newInodeIndex
	copy(newDirBlock.B_content[1].B_name[:], "..")
	newDirBlock.B_content[1].B_inodo = parentInodeIndex

	blockPos := sb.S_block_start + newBlockIndex*int32(binary.Size(Particiones.FolderBlock{}))
	if err := Utils.WriteFile(file, newDirBlock, int64(blockPos)); err != nil {
		return fmt.Errorf("error al escribir el nuevo bloque de carpeta: %v", err)
	}

	// Crear, configurar y escribir el nuevo inodo
	var newInode Particiones.Inode
	currentTime := time.Now().Format("02/01/2006 15:04")
	newInode.I_uid = int32(user.ID)
	newInode.I_gid = int32(user.GID)
	copy(newInode.I_ctime[:], currentTime)
	copy(newInode.I_mtime[:], currentTime)
	newInode.I_type[0] = '0'
	copy(newInode.I_perm[:], "664")
	for i := range newInode.I_block {
		newInode.I_block[i] = -1
	}
	newInode.I_block[0] = newBlockIndex

	inodePos := sb.S_inode_start + newInodeIndex*int32(binary.Size(Particiones.Inode{}))
	if err := Utils.WriteFile(file, newInode, int64(inodePos)); err != nil {
		return fmt.Errorf("error al escribir el nuevo inodo: %v", err)
	}

	// Agregar la entrada al directorio padre
	if err := addEntryToDir(dirName, newInodeIndex, parentInode, parentInodeIndex, file, sb, &part); err != nil {
		return fmt.Errorf("error al agregar entrada al directorio padre: %v", err)
	}

	// Actualizar bitmaps y Superbloque
	setBit(sb.S_bm_inode_start, newInodeIndex, file)
	setBit(sb.S_bm_block_start, newBlockIndex, file)

	sb.S_free_inodes_count--
	sb.S_free_blocks_count--
	if err := Utils.WriteFile(file, sb, int64(part.MountStart)); err != nil {
		return fmt.Errorf("error al actualizar superbloque: %v", err)
	}

	return nil
}

// addEntryToDir busca un espacio libre en los bloques de un directorio padre y añade la nueva entrada.
func addEntryToDir(name string, inodeIdx int32, parentInode Particiones.Inode, parentInodeIndex int32, file *os.File, sb *Particiones.SuperBlock, part *Entornos.MountedPartition) error {
	for i := 0; i < 12; i++ {
		blockIndex := parentInode.I_block[i]
		if blockIndex != -1 {
			var dirBlock Particiones.FolderBlock
			blockPos := sb.S_block_start + blockIndex*int32(binary.Size(Particiones.FolderBlock{}))
			if err := Utils.ReadFile(file, &dirBlock, int64(blockPos)); err != nil {
				continue
			}
			for j := range dirBlock.B_content {
				if dirBlock.B_content[j].B_inodo == -1 {
					dirBlock.B_content[j].B_inodo = inodeIdx
					copy(dirBlock.B_content[j].B_name[:], name)
					return Utils.WriteFile(file, dirBlock, int64(blockPos))
				}
			}
		}
	}

	// Si no hay espacio, asignar un nuevo bloque al inodo padre
	for i := 0; i < 12; i++ {
		if parentInode.I_block[i] == -1 {
			newBlockIndex, err := findFreeBit(sb.S_bm_block_start, sb.S_blocks_count, file)
			if err != nil {
				return fmt.Errorf("no hay bloques libres para expandir el directorio padre")
			}

			parentInode.I_block[i] = newBlockIndex
			parentInodePos := sb.S_inode_start + parentInodeIndex*int32(binary.Size(Particiones.Inode{}))
			if err := Utils.WriteFile(file, parentInode, int64(parentInodePos)); err != nil {
				return fmt.Errorf("no se pudo actualizar el inodo padre con el nuevo bloque")
			}

			setBit(sb.S_bm_block_start, newBlockIndex, file)
			sb.S_free_blocks_count--
			Utils.WriteFile(file, sb, int64(part.MountStart)) // Actualizar SB

			var newDirBlock Particiones.FolderBlock
			for k := range newDirBlock.B_content {
				newDirBlock.B_content[k].B_inodo = -1
			}
			newDirBlock.B_content[0].B_inodo = inodeIdx
			copy(newDirBlock.B_content[0].B_name[:], name)

			blockPos := sb.S_block_start + newBlockIndex*int32(binary.Size(Particiones.FolderBlock{}))
			return Utils.WriteFile(file, newDirBlock, int64(blockPos))
		}
	}

	return fmt.Errorf("el directorio padre está lleno y no se pueden agregar más entradas")
}

// =================================================================================
// HELPERS DE BÚSQUEDA Y BITMAPS (CORREGIDOS)
// =================================================================================

func findFreeBit(bmStart int32, count int32, file *os.File) (int32, error) {
	for i := int32(0); i < count; i++ {
		byteOffset := bmStart + (i / 8)
		bitOffset := i % 8
		var b [1]byte
		if err := Utils.ReadFile(file, &b, int64(byteOffset)); err != nil {
			return -1, fmt.Errorf("error al leer bitmap en byte %d: %v", byteOffset, err)
		}
		if (b[0] & (1 << bitOffset)) == 0 {
			return i, nil
		}
	}
	return -1, fmt.Errorf("no se encontró espacio libre en el bitmap")
}

func setBit(bmStart int32, index int32, file *os.File) error {
	byteOffset := bmStart + (index / 8)
	bitOffset := index % 8
	var b [1]byte
	if err := Utils.ReadFile(file, &b, int64(byteOffset)); err != nil {
		return fmt.Errorf("error al leer byte para actualizar bitmap: %v", err)
	}
	b[0] |= (1 << bitOffset)
	return Utils.WriteFile(file, b, int64(byteOffset))
}

func findInodeByPath(path string, file *os.File, sb *Particiones.SuperBlock) (int32, Particiones.Inode) {
	path = normalizeFSPath(path)
	var inode Particiones.Inode
	if path == "/" {
		if err := Utils.ReadFile(file, &inode, int64(sb.S_inode_start)); err == nil {
			return 0, inode
		}
		return -1, inode
	}

	inodeIndex, _ := Usuarios.InitSearch(path, file, *sb)
	if inodeIndex != -1 {
		inodePos := sb.S_inode_start + inodeIndex*int32(binary.Size(Particiones.Inode{}))
		if err := Utils.ReadFile(file, &inode, int64(inodePos)); err == nil {
			return inodeIndex, inode
		}
	}
	return -1, inode
}

func entryExistsInDir(name string, parentInode Particiones.Inode, file *os.File, sb *Particiones.SuperBlock) bool {
	if parentInode.I_type[0] != '0' {
		return false
	}
	for i := 0; i < 12; i++ {
		blockIndex := parentInode.I_block[i]
		if blockIndex != -1 {
			var dirBlock Particiones.FolderBlock
			blockPos := sb.S_block_start + blockIndex*int32(binary.Size(Particiones.FolderBlock{}))
			if err := Utils.ReadFile(file, &dirBlock, int64(blockPos)); err == nil {
				for _, entry := range dirBlock.B_content {
					entryName := strings.TrimRight(string(entry.B_name[:]), "\x00")
					if entryName == name {
						return true
					}
				}
			}
		}
	}
	return false
}

// =================================================================================
// HELPERS ADICIONALES
// =================================================================================

func normalizeFSPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "." {
		return "/"
	}
	p = filepath.ToSlash(filepath.Clean(p))
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimRight(p, "/")
	}
	return p
}
