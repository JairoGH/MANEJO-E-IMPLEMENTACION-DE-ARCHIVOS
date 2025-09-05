package Estructuras

import (
	"backend/Particiones"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// Función auxiliar para crear el sistema de archivos EXT2
func create_ext2(n int32, partition Particiones.Partition, newSuperblock Particiones.SuperBlock, date string, file *os.File) string {
	var output strings.Builder
	output.WriteString("|============================================================================|\n")
	output.WriteString("|==============================  CREANDO EXT2 ===============================|\n")
	output.WriteString("|============================================================================|\n")
	output.WriteString(fmt.Sprintf("  INODOS: %d \n", n))

	// Imprimir el Superblock calculado
	output.WriteString(Particiones.PrintSuperblock(newSuperblock))
	output.WriteString(fmt.Sprintf("  Date: %s\n", date))

	// Escribir los bitmaps de inodos y bloques
	for i := int32(0); i < n; i++ {
		if err := Utils.WriteFile(file, byte(0), int64(newSuperblock.S_bm_inode_start+i)); err != nil {
			return "Error al escribir en el bitmap de inodos"
		}
	}

	// Escribir los bitmaps de bloques
	for i := int32(0); i < 3*n; i++ {
		if err := Utils.WriteFile(file, byte(0), int64(newSuperblock.S_bm_block_start+i)); err != nil {
			return "Error al escribir en el bitmap de bloques"
		}
	}

	// Inicializar los inodos y bloques con valores predeterminados
	if err := initInodesAndBlocks(n, newSuperblock, file); err != nil {
		return "Error al inicializar inodos y bloques"
	}

	// Crear la carpeta raíz y el archivo "users.txt"
	if err := createRootAndUsersFile(newSuperblock, date, file); err != nil {
		return "Error al crear la carpeta raíz y el archivo users.txt"
	}

	// Escribir el superbloque actualizado en el archivo
	if err := Utils.WriteFile(file, newSuperblock, int64(partition.Part_Start)); err != nil {
		return "Error al escribir el superbloque actualizado en el archivo"
	}

	// Marcar los primeros inodos y bloques como usados
	if err := markUsedInodesAndBlocks(newSuperblock, file); err != nil {
		return "Error al marcar inodos y bloques como usados"
	}

	// Leer e imprimir los inodos después de formatear
	output.WriteString("|===================================================================================|\n")
	output.WriteString("|==============================  IMPRIMIENDO INODOS  ===============================|\n")
	output.WriteString("|===================================================================================|\n")
	for i := int32(0); i < n; i++ {
		var inode Particiones.Inode
		offset := int64(newSuperblock.S_inode_start + i*int32(binary.Size(Particiones.Inode{})))
		if err := Utils.ReadFile(file, &inode, offset); err != nil {
			return fmt.Sprintf("Error al leer inodo: %v", err)
		}
		Particiones.PrintInode(inode)
	}

	// Leer e imprimir los Folderblocks y Fileblocks
	output.WriteString("|===================================================================================|\n")
	output.WriteString("|===========================  FOLDERBLOCKS Y FILEBLOCKS  ===========================|\n")
	output.WriteString("|===================================================================================|\n")

	// Imprimir Folderblocks
	for i := int32(0); i < 1; i++ {
		var folderblock Particiones.FolderBlock
		offset := int64(newSuperblock.S_block_start + i*int32(binary.Size(Particiones.FolderBlock{})))
		if err := Utils.ReadFile(file, &folderblock, offset); err != nil {
			return fmt.Sprintf("Error al leer Folderblock: %v", err)
		}
		output.WriteString(Particiones.PrintFolderBlock(folderblock))
	}

	// Imprimir Fileblocks
	for i := int32(0); i < 1; i++ {
		var fileblock Particiones.FileBlock
		offset := int64(newSuperblock.S_block_start + int32(binary.Size(Particiones.FolderBlock{})) + i*int32(binary.Size(Particiones.FileBlock{})))
		if err := Utils.ReadFile(file, &fileblock, offset); err != nil {
			return fmt.Sprintf("Error al leer Fileblock: %v", err)
		}
		output.WriteString(Particiones.PrintFileBlock(fileblock))
	}

	// Imprimir el Superblock final
	output.WriteString(Particiones.PrintSuperblock(newSuperblock))
	output.WriteString("|===================================================================================|\n")
	output.WriteString("|===============================  FINALIZANDO EXT2  ================================|\n")
	output.WriteString("|===================================================================================|\n")
	return output.String()
}

// Función auxiliar para inicializar inodos y bloques
func initInodesAndBlocks(n int32, newSuperblock Particiones.SuperBlock, file *os.File) error {
	var newInode Particiones.Inode
	for i := int32(0); i < 15; i++ {
		newInode.I_block[i] = -1
	}

	for i := int32(0); i < n; i++ {
		if err := Utils.WriteFile(file, newInode, int64(newSuperblock.S_inode_start+i*int32(binary.Size(Particiones.Inode{})))); err != nil {
			return err
		}
	}

	var newFileblock Particiones.FileBlock
	for i := int32(0); i < 3*n; i++ {
		if err := Utils.WriteFile(file, newFileblock, int64(newSuperblock.S_block_start+i*int32(binary.Size(Particiones.FileBlock{})))); err != nil {
			return err
		}
	}

	return nil
}

// Función auxiliar para crear la carpeta raíz y el archivo users.txt
func createRootAndUsersFile(newSuperblock Particiones.SuperBlock, date string, file *os.File) error {
	var Inode0, Inode1 Particiones.Inode

	// Inicializa los inodos con la fecha proporcionada
	initInode(&Inode0, date)
	initInode(&Inode1, date)

	// Asigna los bloques correspondientes a los inodos
	Inode0.I_block[0] = 0
	Inode1.I_block[0] = 1

	// Contenido del archivo users.txt
	data := "  1,G,root\n  1,U,root,root,123\n"
	actualSize := int32(len(data))
	Inode1.I_size = actualSize

	// Crea un bloque de archivo y copia el contenido en él
	var Fileblock1 Particiones.FileBlock
	copy(Fileblock1.B_content[:], data)

	// Crea un bloque de carpeta (raíz) y asigna las entradas iniciales
	var Folderblock0 Particiones.FolderBlock
	Folderblock0.B_content[0].B_inodo = 0
	copy(Folderblock0.B_content[0].B_name[:], ".")
	Folderblock0.B_content[1].B_inodo = 0
	copy(Folderblock0.B_content[1].B_name[:], "..")
	Folderblock0.B_content[2].B_inodo = 1
	copy(Folderblock0.B_content[2].B_name[:], "users.txt")

	// Escribe los inodos y bloques en las posiciones correctas en el archivo del sistema
	if err := Utils.WriteFile(file, Inode0, int64(newSuperblock.S_inode_start)); err != nil {
		return err
	}
	if err := Utils.WriteFile(file, Inode1, int64(newSuperblock.S_inode_start+int32(binary.Size(Particiones.Inode{})))); err != nil {
		return err
	}
	if err := Utils.WriteFile(file, Folderblock0, int64(newSuperblock.S_block_start)); err != nil {
		return err
	}
	if err := Utils.WriteFile(file, Fileblock1, int64(newSuperblock.S_block_start+int32(binary.Size(Particiones.FolderBlock{})))); err != nil {
		return err
	}

	return nil
}
