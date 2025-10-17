package Estructuras

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

func Mkfs(id string, type_ string, fs_ string) string {
	var output strings.Builder
	output.WriteString(" ========================================================= \n")
	output.WriteString(" ======================= INICIO MKFS ===================== \n")
	output.WriteString(" ========================================================= \n")
	output.WriteString(fmt.Sprintf("  Id: %s\n", id))
	output.WriteString(fmt.Sprintf("  Type: %s\n", type_))
	output.WriteString(fmt.Sprintf("  Fs: %s\n", fs_))

	currentDate := time.Now().Format("02/01/2006 15:04")

	var mountedPartition Entornos.MountedPartition
	var partitionFound bool
	for _, partitions := range Entornos.GetMountedPartitions() {
		for _, partition := range partitions {
			if partition.MountID == id {
				mountedPartition = partition
				partitionFound = true
				break
			}
		}
		if partitionFound {
			break
		}
	}

	if !partitionFound {
		return "Error: Partición no encontrada."
	}
	if mountedPartition.MountStatus != '1' {
		return "Error: La partición aún no está Montada."
	}

	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		return "Error: No se pudo abrir el Archivo Binario de la Partición."
	}
	defer file.Close()

	var TempMBR Particiones.MBR
	if err := Utils.ReadFile(file, &TempMBR, 0); err != nil {
		return "Error: No se pudo leer el MBR del Archivo Binario."
	}

	var partitionIndex int = -1
	for i := 0; i < 4; i++ {
		if strings.Contains(string(TempMBR.MBR_Partition[i].Part_ID[:]), id) {
			partitionIndex = i
			break
		}
	}
	if partitionIndex == -1 {
		return "Error: Partición no encontrada en el MBR."
	}

	partition := TempMBR.MBR_Partition[partitionIndex]

	// ========== CÁLCULO DE ESTRUCTURAS CORRECTO ==========
	// Ecuación basada en el enunciado, asumiendo que el tamaño del bitmap es 1 byte por cada entrada (n y 3n)
	// n = (TamañoPartición - sizeof(Superblock)) / (1_byte_bm_inodo + 3_bytes_bm_bloques + sizeof(Inodo) + 3*sizeof(Block))
	sizeOfInode := int32(binary.Size(Particiones.Inode{}))
	sizeOfBlock := int32(binary.Size(Particiones.FileBlock{}))

	numerador := partition.Part_Size - int32(binary.Size(Particiones.SuperBlock{}))
	denominador := int32(1+3) + sizeOfInode + (3 * sizeOfBlock) // 1 y 3 son los bytes para los bitmaps
	if denominador <= 0 {
		return "Error: El tamaño de la partición es demasiado pequeño para formatear."
	}
	n := numerador / denominador

	output.WriteString(fmt.Sprintf("  INODOS CALCULADOS: %d\n", n))

	// ========== CREACIÓN DEL SUPERBLOQUE CON OFFSETS CORRECTOS ==========
	var newSuperblock Particiones.SuperBlock
	newSuperblock.S_filesystem_type = 2
	newSuperblock.S_inodes_count = n
	newSuperblock.S_blocks_count = 3 * n
	newSuperblock.S_free_blocks_count = (3 * n) - 2
	newSuperblock.S_free_inodes_count = n - 2
	copy(newSuperblock.S_mtime[:], currentDate)
	copy(newSuperblock.S_unmtime[:], currentDate)
	newSuperblock.S_mnt_count = 1
	newSuperblock.S_magic = 0xEF53
	newSuperblock.S_inode_size = sizeOfInode
	newSuperblock.S_block_size = sizeOfBlock
	newSuperblock.S_first_ino = 2 // Inodos 0 y 1 estarán ocupados
	newSuperblock.S_first_blo = 2 // Bloques 0 y 1 estarán ocupados

	// --- ESTA ES LA CORRECCIÓN MÁS IMPORTANTE ---
	newSuperblock.S_bm_inode_start = partition.Part_Start + int32(binary.Size(Particiones.SuperBlock{}))
	newSuperblock.S_bm_block_start = newSuperblock.S_bm_inode_start + n                          // El bitmap de bloques empieza después de los N BYTES del bitmap de inodos
	newSuperblock.S_inode_start = newSuperblock.S_bm_block_start + (3 * n)                       // La tabla de inodos empieza después de los 3*N BYTES del bitmap de bloques
	newSuperblock.S_block_start = newSuperblock.S_inode_start + (n * newSuperblock.S_inode_size) // La tabla de bloques empieza después de (n * tamaño_inodo) bytes

	if fs_ == "2fs" {
		output.WriteString(create_ext2(n, partition, newSuperblock, currentDate, file))
	} else {
		output.WriteString("EXT3 no está soportado.")
	}

	output.WriteString(" ========================================================= \n")
	output.WriteString(" ======================= FIN MKFS ======================= \n")
	output.WriteString(" ========================================================= \n")
	return output.String()
}
