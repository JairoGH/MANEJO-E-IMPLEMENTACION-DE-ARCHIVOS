package Estructuras

import (
	"backend/Particiones"
	"backend/Utils"
	"fmt"
	"os"
	"strings"
)

// create_ext2 ahora usa los offsets correctos y asegura una inicialización limpia.
func create_ext2(n int32, partition Particiones.Partition, newSuperblock Particiones.SuperBlock, date string, file *os.File) string {
	var output strings.Builder
	output.WriteString(" ========================================================= \n")
	output.WriteString(" =====================  CREANDO EXT2 ===================== \n")
	output.WriteString(" ========================================================= \n")

	// Escribir ceros en los bitmaps (representando 'libre')
	bmInodeBytes := make([]byte, n)
	bmBlockBytes := make([]byte, 3*n)
	Utils.WriteFile(file, bmInodeBytes, int64(newSuperblock.S_bm_inode_start))
	Utils.WriteFile(file, bmBlockBytes, int64(newSuperblock.S_bm_block_start))

	// Inicializar todas las tablas de inodos y bloques para asegurar que estén limpias
	initInodesAndBlocks(n, newSuperblock, file)

	// Crear la carpeta raíz "/" y el archivo "/users.txt"
	if err := createRootAndUsersFile(newSuperblock, date, file); err != nil {
		return fmt.Sprintf("Error al crear la estructura raíz: %v", err)
	}

	// Marcar los primeros inodos y bloques (0 y 1) como usados en el bitmap
	if err := markUsedInodesAndBlocks(newSuperblock, file); err != nil {
		return fmt.Sprintf("Error al actualizar bitmaps: %v", err)
	}

	// Escribir el superbloque final en el disco (después de todas las operaciones)
	if err := Utils.WriteFile(file, newSuperblock, int64(partition.Part_Start)); err != nil {
		return fmt.Sprintf("Error al escribir el superbloque final: %v", err)
	}

	output.WriteString(" ✅ Sistema de archivos EXT2 creado correctamente.\n")
	output.WriteString(Particiones.PrintSuperblock(newSuperblock))
	output.WriteString(" =================  FINALIZANDO EXT2  ==================== \n")
	return output.String()
}

func initInodesAndBlocks(n int32, sb Particiones.SuperBlock, file *os.File) error {
	var emptyInode Particiones.Inode
	for i := range emptyInode.I_block {
		emptyInode.I_block[i] = -1
	}
	for i := int32(0); i < n; i++ {
		if err := Utils.WriteFile(file, emptyInode, int64(sb.S_inode_start+i*sb.S_inode_size)); err != nil {
			return err
		}
	}

	var emptyBlock Particiones.FileBlock // Sirve para cualquier tipo de bloque de 64 bytes
	for i := int32(0); i < 3*n; i++ {
		if err := Utils.WriteFile(file, emptyBlock, int64(sb.S_block_start+i*sb.S_block_size)); err != nil {
			return err
		}
	}
	return nil
}

func initInode(inode *Particiones.Inode, date string) {
	inode.I_uid = 1
	inode.I_gid = 1
	inode.I_size = 0
	copy(inode.I_atime[:], date)
	copy(inode.I_ctime[:], date)
	copy(inode.I_mtime[:], date)
	copy(inode.I_perm[:], "664") // Permisos iniciales para la raíz y users.txt

	// El tipo se setea específicamente
	for i := 0; i < 15; i++ {
		inode.I_block[i] = -1
	}
}

func createRootAndUsersFile(sb Particiones.SuperBlock, date string, file *os.File) error {
	var inode0, inode1 Particiones.Inode

	// Inodo 0: Carpeta Raíz "/"
	initInode(&inode0, date)
	inode0.I_type[0] = '0' // 0 = Directorio
	inode0.I_block[0] = 0

	// Inodo 1: Archivo "/users.txt"
	data := "1,G,root\n1,U,root,root,123\n"
	initInode(&inode1, date)
	inode1.I_type[0] = '1' // 1 = Archivo
	inode1.I_size = int32(len(data))
	inode1.I_block[0] = 1

	// Bloque 0: Contenido de la Carpeta Raíz
	var folderBlock0 Particiones.FolderBlock
	// --- CORRECCIÓN IMPORTANTE: Inicializar entradas no usadas ---
	for i := range folderBlock0.B_content {
		folderBlock0.B_content[i].B_inodo = -1
	}
	copy(folderBlock0.B_content[0].B_name[:], ".")
	folderBlock0.B_content[0].B_inodo = 0
	copy(folderBlock0.B_content[1].B_name[:], "..")
	folderBlock0.B_content[1].B_inodo = 0 // La raíz es su propio padre
	copy(folderBlock0.B_content[2].B_name[:], "users.txt")
	folderBlock0.B_content[2].B_inodo = 1

	// Bloque 1: Contenido del Archivo "/users.txt"
	var fileBlock1 Particiones.FileBlock
	copy(fileBlock1.B_content[:], data)

	// Escribir todo en el disco
	if err := Utils.WriteFile(file, inode0, int64(sb.S_inode_start)); err != nil {
		return err
	}
	if err := Utils.WriteFile(file, inode1, int64(sb.S_inode_start+sb.S_inode_size)); err != nil {
		return err
	}
	if err := Utils.WriteFile(file, folderBlock0, int64(sb.S_block_start)); err != nil {
		return err
	}
	if err := Utils.WriteFile(file, fileBlock1, int64(sb.S_block_start+sb.S_block_size)); err != nil {
		return err
	}

	return nil
}
