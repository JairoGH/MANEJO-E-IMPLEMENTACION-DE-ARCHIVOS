package Particiones

import (
	"fmt"
	"strings"
)

// Estructura del MBR
type MBR struct {
	MBR_Tamano    int32
	MBR_FechaCr   [19]byte
	MBR_DiskSig   int32
	MBR_DiskFit   [1]byte
	MBR_Partition [4]Partition
}

// Estructura de la partición primaria o extendida
type Partition struct {
	Part_Status      [1]byte
	Part_Type        [1]byte
	Part_Fit         [1]byte
	Part_Start       int32
	Part_Size        int32
	Part_Name        [16]byte
	Part_Correlative int32
	Part_ID          [4]byte
}

// Estructura de la partición lógica (EBR)
type EBR struct {
	Part_Mount byte
	Part_Fit   byte
	Part_Start int32
	Part_Size  int32
	Part_Next  int32
	Part_Name  [16]byte
}

// Estructura del Superblock
type SuperBlock struct {
	S_filesystem_type   int32
	S_inodes_count      int32
	S_blocks_count      int32
	S_free_blocks_count int32
	S_free_inodes_count int32
	S_mtime             [17]byte
	S_unmtime           [17]byte
	S_mnt_count         int32
	S_magic             int32
	S_inode_size        int32
	S_block_size        int32
	S_first_ino         int32
	S_first_blo         int32
	S_bm_inode_start    int32
	S_bm_block_start    int32
	S_inode_start       int32
	S_block_start       int32
}

// Estructura del Inodo
type Inode struct {
	I_uid   int32
	I_gid   int32
	I_size  int32
	I_atime [17]byte
	I_ctime [17]byte
	I_mtime [17]byte
	I_block [15]int32
	I_type  [1]byte
	I_perm  [3]byte
}

type Content struct {
	B_name  [12]byte
	B_inodo int32
}

type BlockPointer struct {
	B_pointer [16]int32
}

type FolderBlock struct {
	B_content [4]Content
}

type FileBlock struct {
	B_content [64]byte
}

// Funcion para Imprimir el MBR
func PrintMBR(mbr MBR) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf(" Signature: %d", mbr.MBR_DiskSig))
	output.WriteString(fmt.Sprintf("Fecha de creación: %s", string(mbr.MBR_FechaCr[:])))
	output.WriteString(fmt.Sprintf("Tamaño: %d bytes", mbr.MBR_Tamano))
	output.WriteString(fmt.Sprintf("Ajuste: %s", string(mbr.MBR_DiskFit[:])))

	return output.String()
}

// Funcion para Imprimir la partición
func PrintPartition(part Partition) string {

	var output strings.Builder

	output.WriteString(fmt.Sprintf(" Nombre: %s", string(part.Part_Name[:])))
	output.WriteString(fmt.Sprintf(" Type: %s", string(part.Part_Type[:])))
	output.WriteString(fmt.Sprintf(" Start: %d", part.Part_Start))
	output.WriteString(fmt.Sprintf(" Size: %d", part.Part_Size))
	output.WriteString(fmt.Sprintf(" Status: %s", string(part.Part_Status[:])))
	output.WriteString(fmt.Sprintf(" ID: %s", string(part.Part_ID[:])))

	return output.String()
}

// Funcion para Imprimir la EBR
func PrintEBR(ebr EBR) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf(" Nombre: %s", string(ebr.Part_Name[:])))
	output.WriteString(fmt.Sprintf(" Fit: %c", ebr.Part_Fit))
	output.WriteString(fmt.Sprintf(" Start: %d", ebr.Part_Start))
	output.WriteString(fmt.Sprintf(" Size: %d", ebr.Part_Size))
	output.WriteString(fmt.Sprintf(" Next: %d", ebr.Part_Next))
	output.WriteString(fmt.Sprintf(" Mount: %c", ebr.Part_Mount))

	return output.String()
}

// Funcion para Imprimir el Superblock
func PrintSuperblock(sb SuperBlock) string {
	var output strings.Builder
	output.WriteString(" ======================= SUPERBLOCK ==========================")
	output.WriteString(fmt.Sprintf(" S_filesystem_type: %d\n", sb.S_filesystem_type))
	output.WriteString(fmt.Sprintf(" S_inodes_count: %d\n", sb.S_inodes_count))
	output.WriteString(fmt.Sprintf(" S_blocks_count: %d\n", sb.S_blocks_count))
	output.WriteString(fmt.Sprintf(" S_free_blocks_count: %d\n", sb.S_free_blocks_count))
	output.WriteString(fmt.Sprintf(" S_free_inodes_count: %d\n", sb.S_free_inodes_count))
	output.WriteString(fmt.Sprintf(" S_mtime: %s\n", string(sb.S_mtime[:])))
	output.WriteString(fmt.Sprintf(" S_unmtime: %s\n", string(sb.S_unmtime[:])))
	output.WriteString(fmt.Sprintf(" S_mnt_count: %d\n", sb.S_mnt_count))
	output.WriteString(fmt.Sprintf(" S_magic: %d\n", sb.S_magic))
	output.WriteString(fmt.Sprintf(" S_inode_size: %d\n", sb.S_inode_size))
	output.WriteString(fmt.Sprintf(" S_block_size: %d\n", sb.S_block_size))
	output.WriteString(fmt.Sprintf(" S_first_ino: %d\n", sb.S_first_ino))
	output.WriteString(fmt.Sprintf(" S_first_blo: %d\n", sb.S_first_blo))
	output.WriteString(fmt.Sprintf(" S_bm_inode_start: %d\n", sb.S_bm_inode_start))
	output.WriteString(fmt.Sprintf(" S_bm_block_start: %d\n", sb.S_bm_block_start))
	output.WriteString(fmt.Sprintf(" S_inode_start: %d\n", sb.S_inode_start))
	output.WriteString(fmt.Sprintf(" S_block_start: %d\n", sb.S_block_start))
	output.WriteString(" ============================================================ ")
	return output.String()
}

// Funcion para Imprimir el Inodo
func PrintInode(inode Inode) string {
	var output strings.Builder

	output.WriteString(" ======================= INODO ======================= \n")
	output.WriteString(fmt.Sprintf(" I_uid: %d\n", inode.I_uid))
	output.WriteString(fmt.Sprintf(" I_gid: %d\n", inode.I_gid))
	output.WriteString(fmt.Sprintf(" I_size: %d\n", inode.I_size))
	output.WriteString(fmt.Sprintf(" I_atime: %s\n", string(inode.I_atime[:])))
	output.WriteString(fmt.Sprintf(" I_ctime: %s\n", string(inode.I_ctime[:])))
	output.WriteString(fmt.Sprintf(" I_mtime: %s\n", string(inode.I_mtime[:])))
	output.WriteString(fmt.Sprintf(" I_type: %s\n", string(inode.I_type[:])))
	output.WriteString(fmt.Sprintf(" I_perm: %s\n", string(inode.I_perm[:])))
	output.WriteString(fmt.Sprintf(" I_block: %v\n", inode.I_block))
	output.WriteString(" ===================================================== \n")
	return output.String()
}

// Funcion para Imprimir el Bloque de Carpeta
func PrintFolderBlock(folderblock FolderBlock) string {
	var output strings.Builder
	output.WriteString(" ======================= BLOQUE DE CARPETA =======================n")
	for i, content := range folderblock.B_content {
		output.WriteString(fmt.Sprintf(" Contenido [%d]: Nombre: %s, Inodo: %d\n", i, string(content.B_name[:]), content.B_inodo))
	}
	output.WriteString(" =============================================================== \n")
	return output.String()
}

// Funcion para Imprimir Bloque de Archivos
func PrintFileBlock(fileblock FileBlock) string {
	var output strings.Builder
	output.WriteString(" ======================= BLOQUE DE ARCHIVOS ======================= \n")
	output.WriteString(fmt.Sprintf(" B_Content: %s\n", string(fileblock.B_content[:])))
	output.WriteString(" =================================================================  \n")
	return output.String()
}

// Funcion para Imprimir Bloque de Apuntadores
func PrintBlockPointer(blockpointer BlockPointer) string {
	var output strings.Builder
	output.WriteString(" ======================= BLOQUE DE APUNTADORES ======================= \n")
	for i, pointer := range blockpointer.B_pointer {
		output.WriteString(fmt.Sprintf(" Pointer [%d]: %d\n", i, pointer))
	}
	output.WriteString(" =================================================================== \n")
	return output.String()
}
