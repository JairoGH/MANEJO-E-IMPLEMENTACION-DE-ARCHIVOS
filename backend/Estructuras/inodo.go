package Estructuras

import (
	"backend/Particiones"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// Función auxiliar para inicializar un inodo
func initInode(inode *Particiones.Inode, date string) {
	inode.I_uid = 1
	inode.I_gid = 1
	inode.I_size = 0
	copy(inode.I_atime[:], date)
	copy(inode.I_ctime[:], date)
	copy(inode.I_mtime[:], date)
	copy(inode.I_perm[:], "664")

	for i := int32(0); i < 15; i++ {
		inode.I_block[i] = -1
	}
}

// GetInodeFileData lee el contenido de un archivo a partir de su inodo.
func GetInodeFileData(inode Particiones.Inode, file *os.File, superblock Particiones.SuperBlock) (string, string) {
	var output strings.Builder
	var content string

	output.WriteString(" ============================================================================================= \n")
	output.WriteString(" ==================================  CONTENIDO DEL BLOQUE   ================================== \n")
	output.WriteString(" ============================================================================================= \n")
	index := int32(0)

	// Iterar sobre los bloques del inodo
	for _, block := range inode.I_block {
		if block != -1 {
			// Manejo de bloques directos (0-12)
			if index < 13 {
				var crrFileblock Particiones.FileBlock

				// Leer el bloque de archivo desde el archivo binario
				if err := Utils.ReadFile(file, &crrFileblock, int64(superblock.S_block_start+block*int32(binary.Size(Particiones.FileBlock{})))); err != nil {
					output.WriteString(fmt.Sprintf(" Error al leer el bloque de archivo: %v\n", err))
					return "", output.String()
				}

				// Concatenar el contenido del bloque
				content += string(crrFileblock.B_content[:])
				output.WriteString(fmt.Sprintf(" Bloque leído: %d, Contenido: %s\n", block, string(crrFileblock.B_content[:])))
			} else {
				output.WriteString(" Manejo de bloques indirectos no implementado\n") // Manejo de bloques indirectos
			}
		}
		index++
	}

	output.WriteString(" ============================================================================================= \n")
	output.WriteString(" ==================================  FIN CONTENIDO DEL BLOQUE  =============================== \n")
	output.WriteString(" ============================================================================================= \n")
	return content, output.String()
}

func PrintInode(inode Particiones.Inode) string {
	var output strings.Builder
	output.WriteString(" ============================================================================================= \n")
	output.WriteString(" =====================================   INODE   ============================================= \n")
	output.WriteString(" ============================================================================================= \n")
	output.WriteString(fmt.Sprintf(" I_uid: %d\n", inode.I_uid))
	output.WriteString(fmt.Sprintf(" I_gid: %d\n", inode.I_gid))
	output.WriteString(fmt.Sprintf(" I_size: %d\n", inode.I_size))
	output.WriteString(fmt.Sprintf(" I_atime: %s\n", string(inode.I_atime[:])))
	output.WriteString(fmt.Sprintf(" I_ctime: %s\n", string(inode.I_ctime[:])))
	output.WriteString(fmt.Sprintf(" I_mtime: %s\n", string(inode.I_mtime[:])))
	output.WriteString(fmt.Sprintf(" I_type: %s\n", string(inode.I_type[:])))
	output.WriteString(fmt.Sprintf(" I_perm: %s\n", string(inode.I_perm[:])))
	output.WriteString(fmt.Sprintf(" I_block: %v\n", inode.I_block))
	output.WriteString(" ============================================================================================= \n")
	return output.String()
}
