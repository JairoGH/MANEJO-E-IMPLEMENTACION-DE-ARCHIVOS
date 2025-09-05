package Usuarios

import (
	"backend/Particiones"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

func InitSearch(path string, file *os.File, tempSuperblock Particiones.SuperBlock) (int32, string) {
	var output strings.Builder
	output.WriteString(" ================================================================================================================ \n")
	output.WriteString(" ==============================================  BUSQUEDA INICIAL  ============================================== \n")
	output.WriteString(" ================================================================================================================ \n")

	output.WriteString(fmt.Sprintf("  path: %s\n", path))

	// Dividir la ruta en partes usando "/" como separador
	TempStepsPath := strings.Split(path, "/")
	StepsPath := TempStepsPath[1:]

	output.WriteString(fmt.Sprintf("  StepsPath: %v, len(StepsPath): %d\n", StepsPath, len(StepsPath)))
	for _, step := range StepsPath {
		output.WriteString(fmt.Sprintf("  step: %s\n", step))
	}

	var Inode0 Particiones.Inode
	// Leer el inodo raíz
	if err := Utils.ReadFile(file, &Inode0, int64(tempSuperblock.S_inode_start)); err != nil {
		output.WriteString(fmt.Sprintf(" Error al leer el inodo raíz: %v\n", err))
		return -1, output.String()
	}
	output.WriteString(" ================================================================================================================ \n")
	output.WriteString(" =============================================  FIN BUSQUEDA   ============================================= \n")
	output.WriteString(" ================================================================================================================ \n")

	// Llamar a la función que busca el inodo del archivo según la ruta
	inode, searchLog := SearchInodeByPath(StepsPath, Inode0, file, tempSuperblock)
	output.WriteString(searchLog)

	return inode, output.String()
}

// stack
func pop(s *[]string) string {
	lastIndex := len(*s) - 1
	last := (*s)[lastIndex]
	*s = (*s)[:lastIndex]
	return last
}

func SearchInodeByPath(StepsPath []string, Inode Particiones.Inode, file *os.File, tempSuperblock Particiones.SuperBlock) (int32, string) {
	var output strings.Builder
	output.WriteString(" ================================================================================================================ \n")
	output.WriteString(" =============================================  BUSQUEDA INODO POR PATH   ============================================= \n")
	output.WriteString(" ================================================================================================================ \n")

	index := int32(0)

	// Extrae el primer elemento del path y elimina espacios en blanco
	SearchedName := strings.Replace(pop(&StepsPath), " ", "", -1)

	output.WriteString(fmt.Sprintf(" ======================= SearchedName: %s\n", SearchedName))

	// Iterar sobre los bloques del inodo
	for _, block := range Inode.I_block {
		if block != -1 {
			if index < 13 {
				var crrFolderBlock Particiones.FolderBlock

				// Leer el bloque de carpeta desde el archivo binario
				if err := Utils.ReadFile(file, &crrFolderBlock, int64(tempSuperblock.S_block_start+block*int32(binary.Size(Particiones.FolderBlock{})))); err != nil {
					output.WriteString(fmt.Sprintf(" Error al leer el bloque de carpeta: %v\n", err))
					return -1, output.String()
				}

				// Buscar el archivo/directorio dentro del bloque de carpeta
				for _, folder := range crrFolderBlock.B_content {
					output.WriteString(fmt.Sprintf(" ======================= Folder Name: %s, B_inodo: %d\n", string(folder.B_name[:]), folder.B_inodo))

					// Si el nombre del archivo o directorio coincide
					if strings.Contains(string(folder.B_name[:]), SearchedName) {
						output.WriteString(fmt.Sprintf("\tlen(StepsPath): %d, StepsPath: %v\n", len(StepsPath), StepsPath))

						if len(StepsPath) == 0 {
							output.WriteString(" ======================= Folder found ======================= \n")
							return folder.B_inodo, output.String()
						} else {
							output.WriteString(" ======================= NextInode ======================= \n")
							var NextInode Particiones.Inode

							if err := Utils.ReadFile(file, &NextInode, int64(tempSuperblock.S_inode_start+folder.B_inodo*int32(binary.Size(Particiones.Inode{})))); err != nil {
								output.WriteString(fmt.Sprintf(" Error al leer el siguiente inodo: %v\n", err))
								return -1, output.String()
							}

							// Llamada recursiva para seguir con la búsqueda
							return SearchInodeByPath(StepsPath, NextInode, file, tempSuperblock)
						}
					}
				}
			} else {
				output.WriteString(" Manejo de bloques indirectos no implementado\n")
			}
		}
		index++
	}
	output.WriteString(" ================================================================================================================ \n")
	output.WriteString(" =============================================  FIN BUSQUEDA INODO POR PATH   ============================================= \n")
	output.WriteString(" ================================================================================================================ \n")
	return 0, output.String()
}

// GetInodeFileData lee el contenido de un archivo a partir de su inodo.
func GetInodeFileData(Inode Particiones.Inode, file *os.File, tempSuperblock Particiones.SuperBlock) (string, string) {
	var output strings.Builder
	var content string

	output.WriteString(" ================================================================================================================ \n")
	output.WriteString(" =============================================  CONTENIDO DEL BLOQUE   ============================================= \n")
	output.WriteString(" ================================================================================================================ \n")

	index := int32(0)
	processedBlocks := make(map[int32]bool) // Mapa para rastrear bloques procesados

	// Iterar sobre los bloques del inodo
	for _, block := range Inode.I_block {
		if block != -1 {
			// Verificar si el bloque ya fue procesado
			if processedBlocks[block] {
				output.WriteString(fmt.Sprintf("Bloque %d ya procesado, omitiendo...\n", block))
				continue
			}

			// Manejo de bloques directos (0-12)
			if index < 13 {
				var crrFileBlock Particiones.FileBlock

				// Leer el bloque de archivo desde el archivo binario
				if err := Utils.ReadFile(file, &crrFileBlock, int64(tempSuperblock.S_block_start+block*int32(binary.Size(Particiones.FileBlock{})))); err != nil {
					output.WriteString(fmt.Sprintf("Error al leer el bloque de archivo: %v\n", err))
					return "", output.String()
				}

				// Mostrar el contenido del bloque
				output.WriteString(fmt.Sprintf(", %d, %s\n", block, string(crrFileBlock.B_content[:])))

				// Agregar el contenido del bloque al resultado final
				content += string(crrFileBlock.B_content[:])

				// Marcar el bloque como procesado
				processedBlocks[block] = true
			} else {
				output.WriteString(" Manejo de bloques indirectos no implementado \n")
			}
		}
		index++
	}
	output.WriteString(" ================================================================================================================ \n")
	output.WriteString(" =============================================  FIN CONTENIDO DEL BLOQUE   ============================================= \n")
	output.WriteString(" ================================================================================================================ \n")
	return content, output.String()
}

func AppendToFileBlock(inode *Particiones.Inode, newData string, file *os.File, superblock Particiones.SuperBlock) (error, string) {
	var output strings.Builder
	output.WriteString(" ================================================================================================================ \n")
	output.WriteString(" =============================================   AGREGAR AL BLOQUE    ============================================= \n")
	output.WriteString(" ================================================================================================================ \n")

	// Obtener el contenido actual del archivo
	existingData, log := GetInodeFileData(*inode, file, superblock)
	output.WriteString(log)
	output.WriteString("🔹 Contenido actual de users.txt:\n")
	output.WriteString(existingData + "\n")

	// Unir los datos en un solo string
	fullData := existingData + newData
	output.WriteString(fmt.Sprintf("🔹 Nuevo contenido tras agregar: %s\n", newData))

	// Tamaño de un bloque
	blockSize := binary.Size(Particiones.FileBlock{})

	// Obtener el índice del último bloque usado
	lastBlockIndex := -1
	for i := 0; i < len(inode.I_block); i++ {
		if inode.I_block[i] != -1 {
			lastBlockIndex = i
		} else {
			break
		}
	}
	output.WriteString(fmt.Sprintf("🔹 Último bloque usado en inode: %d\n", lastBlockIndex))

	// Si no hay bloques, asignamos el primero
	if lastBlockIndex == -1 {
		newBlockIndex, log := findFreeBlock(file, superblock)
		output.WriteString(log)
		if newBlockIndex == -1 {
			output.WriteString("❌ Error: No hay bloques libres disponibles\n")
			return fmt.Errorf("no hay bloques libres disponibles"), output.String()
		}
		inode.I_block[0] = int32(newBlockIndex)
		lastBlockIndex = 0
	}

	// Obtener el bloque actual donde se escribe
	blockOffset := int64(superblock.S_block_start + inode.I_block[lastBlockIndex]*int32(blockSize))

	var fileBlock Particiones.FileBlock

	// Leer el bloque actual
	if err := Utils.ReadFile(file, &fileBlock, blockOffset); err != nil {
		output.WriteString(fmt.Sprintf("❌ Error al leer el bloque de archivo: %v\n", err))
		return err, output.String()
	}

	// Verificar cuánto espacio libre queda en el bloque actual
	existingContent := strings.TrimRight(string(fileBlock.B_content[:]), "\x00")
	remainingSpace := blockSize - len(existingContent)

	// Si hay espacio, escribir en el mismo bloque
	if len(newData) <= remainingSpace {
		copy(fileBlock.B_content[len(existingContent):], newData)
	} else {
		// Si no hay suficiente espacio, escribir lo que cabe y manejar el resto
		copy(fileBlock.B_content[len(existingContent):], newData[:remainingSpace])
		newData = newData[remainingSpace:]

		// Asignar un nuevo bloque para el resto de los datos
		newBlockIndex, log := findFreeBlock(file, superblock)
		output.WriteString(log)
		if newBlockIndex == -1 {
			output.WriteString("❌ Error: No hay bloques libres disponibles para el resto de los datos\n")
			return fmt.Errorf("no hay bloques libres disponibles para el resto de los datos"), output.String()
		}
		inode.I_block[lastBlockIndex+1] = int32(newBlockIndex)
		blockOffset = int64(superblock.S_block_start + int32(newBlockIndex)*int32(blockSize))

		// Crear un nuevo bloque y escribir el resto de los datos
		var newFileBlock Particiones.FileBlock
		copy(newFileBlock.B_content[:], newData)
		if err := Utils.WriteFile(file, newFileBlock, blockOffset); err != nil {
			output.WriteString(fmt.Sprintf("❌ Error al escribir el nuevo bloque de archivo: %v\n", err))
			return err, output.String()
		}
	}

	// Escribir el bloque actualizado en el archivo
	if err := Utils.WriteFile(file, fileBlock, blockOffset); err != nil {
		output.WriteString(fmt.Sprintf("❌ Error al escribir el bloque de archivo: %v\n", err))
		return err, output.String()
	}

	// Actualizar el tamaño del inodo
	inode.I_size = int32(len(fullData))
	inodeOffset := int64(superblock.S_inode_start + inode.I_block[0]*int32(binary.Size(Particiones.Inode{})))

	if err := Utils.WriteFile(file, *inode, inodeOffset); err != nil {
		output.WriteString(fmt.Sprintf("❌ Error al actualizar el inodo: %v\n", err))
		return err, output.String()
	}

	output.WriteString(" ================================================================================================================ \n")
	output.WriteString(" ==================================   FIN AGREGAR AL BLOQUE   ======================================= \n")
	output.WriteString(" ================================================================================================================ \n")

	return nil, output.String()
}

func findFreeBlock(file *os.File, superblock Particiones.SuperBlock) (int32, string) {
	var output strings.Builder
	var blockBitmap []byte = make([]byte, superblock.S_blocks_count)

	output.WriteString(" ================================================================================================================ \n")
	output.WriteString(" ==================================   BUSCANDO BLOQUE LIBRE   ======================================= \n")
	output.WriteString(" ================================================================================================================ \n")

	// Leer el bitmap de bloques
	if err := Utils.ReadFile(file, &blockBitmap, int64(superblock.S_bm_block_start)); err != nil {
		output.WriteString(fmt.Sprintf("❌ Error al leer el bitmap de bloques: %v\n", err))
		return -1, output.String()
	}

	// Buscar el primer bloque libre
	for i, b := range blockBitmap {
		if b == 0 {
			// Marcar el bloque como usado
			blockBitmap[i] = 1
			if err := Utils.WriteFile(file, blockBitmap, int64(superblock.S_bm_block_start)); err != nil {
				output.WriteString(fmt.Sprintf("❌ Error al actualizar el bitmap de bloques: %v\n", err))
				return -1, output.String()
			}
			output.WriteString(fmt.Sprintf("✅ Bloque libre encontrado: %d\n", i))
			return int32(i), output.String()
		}
	}

	output.WriteString("❌ No se encontraron bloques libres disponibles\n")
	return -1, output.String()
}
