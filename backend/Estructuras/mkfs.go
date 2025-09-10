package Estructuras

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Usuarios"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"os"
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

	// Obtener la fecha actual en formato día/mes/año
	currentDate := time.Now().Format("02/01/2006")

	// Buscar la partición montada por ID
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

	// Verificar si la partición está montada
	if mountedPartition.MountStatus != '1' {
		return "Error: La partición aún no está Montada."
	}

	// Abrir el archivo binario de la partición
	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		return "Error: No se pudo abrir el Archivo Binario de la Partición."
	}

	// Leer el MBR (Master Boot Record) del archivo binario
	var TempMBR Particiones.MBR
	if err := Utils.ReadFile(file, &TempMBR, 0); err != nil {
		return "Error: No se pudo leer el MBR del Archivo Binario."
	}

	// Buscar la partición dentro del MBR que coincida con el ID proporcionado
	var index int = -1
	for i := 0; i < 4; i++ {
		if TempMBR.MBR_Partition[i].Part_Size != 0 {
			if strings.Contains(string(TempMBR.MBR_Partition[i].Part_ID[:]), id) {
				index = i
				break
			}
		}
	}

	if index == -1 {
		return "  Particion no Encontrada "
	}

	// Calcular el número de inodos basado en el tamaño de la partición
	numerador := int32(TempMBR.MBR_Partition[index].Part_Size - int32(binary.Size(Particiones.SuperBlock{})))
	denominador_base := int32(4 + int32(binary.Size(Particiones.Inode{})) + 3*int32(binary.Size(Particiones.FileBlock{})))
	var temp int32 = 0
	if fs_ == "2fs" {
		temp = 0
	} else {
		output.WriteString("  Error por el momento solo está disponible 2FS.")
	}
	denominador := denominador_base + temp
	n := int32(numerador / denominador)

	output.WriteString(fmt.Sprintf("  INODOS: %d\n", n))

	// Crear el Superblock con los campos calculados
	var newSuperblock Particiones.SuperBlock
	newSuperblock.S_filesystem_type = 2 // EXT2
	newSuperblock.S_inodes_count = n
	newSuperblock.S_blocks_count = 3 * n
	newSuperblock.S_free_blocks_count = 3*n - 2
	newSuperblock.S_free_inodes_count = n - 2
	copy(newSuperblock.S_mtime[:], currentDate)
	copy(newSuperblock.S_unmtime[:], currentDate)
	newSuperblock.S_mnt_count = 1
	newSuperblock.S_magic = 0xEF53
	newSuperblock.S_inode_size = int32(binary.Size(Particiones.Inode{}))
	newSuperblock.S_block_size = int32(binary.Size(Particiones.FileBlock{}))

	// Calcular las posiciones de inicio de los bloques en el disco
	newSuperblock.S_bm_inode_start = TempMBR.MBR_Partition[index].Part_Start + int32(binary.Size(Particiones.SuperBlock{}))
	newSuperblock.S_bm_block_start = newSuperblock.S_bm_inode_start + n
	newSuperblock.S_inode_start = newSuperblock.S_bm_block_start + 3*n
	newSuperblock.S_block_start = newSuperblock.S_inode_start + n*newSuperblock.S_inode_size

	// Crear el sistema de archivos EXT2
	if fs_ == "2fs" {
		output.WriteString(create_ext2(n, TempMBR.MBR_Partition[index], newSuperblock, currentDate, file))
	} else {
		output.WriteString("EXT3 no está soportado.")
	}

	defer file.Close()

	output.WriteString(" ========================================================= \n")
	output.WriteString(" ======================= FIN MKFS ======================= \n")
	output.WriteString(" ========================================================= \n")
	return output.String()
}

// Función para leer y mostrar el contenido de users.txt
func PrintUsersTxt(file *os.File, tempSuperblock Particiones.SuperBlock) {
	fmt.Println(" ======================= Contenido de users.txt ======================= ")

	// Buscar el archivo "users.txt" en el sistema de archivos
	indexInode, log := Usuarios.InitSearch("/users.txt", file, tempSuperblock)
	fmt.Println(log) // Mostrar el log de InitSearch
	if indexInode == -1 {
		fmt.Println("Error: No se encontró el archivo users.txt.")
		return
	}

	// Leer el inodo del archivo "users.txt"
	var crrInode Particiones.Inode
	if err := Utils.ReadFile(file, &crrInode, int64(tempSuperblock.S_inode_start+indexInode*int32(binary.Size(Particiones.Inode{})))); err != nil {
		fmt.Println("Error al leer el inodo de users.txt:", err)
		return
	}

	// Obtener el contenido del archivo "users.txt"
	data, err := GetInodeFileData(crrInode, file, tempSuperblock)
	if err != "" {
		fmt.Println("Error al obtener los datos del inodo de users.txt:", err)
		return
	}

	// Mostrar el contenido de users.txt
	fmt.Println(data)
	fmt.Println(" ======================= Fin del contenido de users.txt ======================= ")

}
