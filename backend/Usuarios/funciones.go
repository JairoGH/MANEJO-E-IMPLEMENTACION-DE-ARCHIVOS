package Usuarios

import (
	"backend/Particiones"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// InitSearch inicia la búsqueda de un inodo a partir de una ruta.
// Esta función ahora llama a la nueva lógica de búsqueda iterativa.
func InitSearch(path string, file *os.File, sb Particiones.SuperBlock) (int32, string) {
	var output strings.Builder
	output.WriteString(" ====================== BÚSQUEDA INICIAL ====================== \n")
	output.WriteString(fmt.Sprintf("  Buscando path: %s\n", path))

	// La raíz es un caso especial, siempre es el inodo 0.
	if path == "/" {
		output.WriteString("  Ruta es raíz, devolviendo inodo 0.\n")
		return 0, output.String()
	}

	// Limpiar y dividir la ruta en componentes. Ej: "/home/user" -> ["home", "user"]
	cleanedPath := strings.Trim(path, "/")
	pathComponents := strings.Split(cleanedPath, "/")

	// La búsqueda comienza desde el inodo raíz (0).
	inodeIndex, log := searchIterative(pathComponents, file, sb)
	output.WriteString(log)

	if inodeIndex == -1 {
		output.WriteString(fmt.Sprintf("  Path '%s' no encontrado.\n", path))
	} else {
		output.WriteString(fmt.Sprintf("  Path '%s' encontrado en inodo %d.\n", path, inodeIndex))
	}
	output.WriteString(" ==================== FIN BÚSQUEDA INICIAL ==================== \n")

	return inodeIndex, output.String()
}

// searchIterative es la nueva función de búsqueda, corregida y no recursiva.
func searchIterative(pathComponents []string, file *os.File, sb Particiones.SuperBlock) (int32, string) {
	var output strings.Builder
	var currentInode Particiones.Inode
	var currentInodeIndex int32 = 0 // Empezar siempre desde la raíz

	// Leer el inodo raíz
	if err := Utils.ReadFile(file, &currentInode, int64(sb.S_inode_start)); err != nil {
		output.WriteString(fmt.Sprintf("  Error crítico: No se pudo leer el inodo raíz (0): %v\n", err))
		return -1, output.String()
	}

	// Recorrer cada parte de la ruta (ej: "home", luego "user")
	for _, component := range pathComponents {
		if component == "" {
			continue
		}

		output.WriteString(fmt.Sprintf("\n  Buscando componente: '%s' en inodo %d\n", component, currentInodeIndex))

		// Solo podemos buscar dentro de directorios
		if currentInode.I_type[0] != '0' {
			output.WriteString(fmt.Sprintf("  Error: Se intentó buscar dentro de un archivo (inodo %d no es un directorio).\n", currentInodeIndex))
			return -1, output.String()
		}

		foundNextComponent := false
		nextInodeIndex := int32(-1)

		// Iterar sobre los bloques directos del inodo actual
		for i := 0; i < 12 && !foundNextComponent; i++ {
			blockIndex := currentInode.I_block[i]
			if blockIndex == -1 {
				continue
			}

			var folderBlock Particiones.FolderBlock
			blockPos := sb.S_block_start + blockIndex*sb.S_block_size
			if err := Utils.ReadFile(file, &folderBlock, int64(blockPos)); err != nil {
				output.WriteString(fmt.Sprintf("  Advertencia: No se pudo leer el bloque de carpeta %d.\n", blockIndex))
				continue
			}

			// Buscar la entrada en el bloque de carpeta
			for _, entry := range folderBlock.B_content {
				entryName := strings.TrimRight(string(entry.B_name[:]), "\x00")
				// --- CORRECCIÓN CLAVE: Comparación exacta en lugar de Contains ---
				if entryName == component {
					output.WriteString(fmt.Sprintf("    -> ¡Encontrado! '%s' apunta al inodo %d.\n", entryName, entry.B_inodo))
					nextInodeIndex = entry.B_inodo
					foundNextComponent = true
					break // Salir del bucle de entradas
				}
			}
		}

		// Si después de buscar en todos los bloques no se encontró el componente
		if !foundNextComponent {
			output.WriteString(fmt.Sprintf("  Error: Componente '%s' no encontrado en el inodo %d.\n", component, currentInodeIndex))
			return -1, output.String() // --- CORRECCIÓN CLAVE: Devolver -1 en caso de fallo ---
		}

		// Preparar para la siguiente iteración: leer el siguiente inodo
		currentInodeIndex = nextInodeIndex
		inodePos := sb.S_inode_start + currentInodeIndex*sb.S_inode_size
		if err := Utils.ReadFile(file, &currentInode, int64(inodePos)); err != nil {
			output.WriteString(fmt.Sprintf("  Error crítico: No se pudo leer el siguiente inodo (%d): %v\n", currentInodeIndex, err))
			return -1, output.String()
		}
	}

	// Si el bucle termina, significa que se recorrió toda la ruta con éxito.
	return currentInodeIndex, output.String()
}

// GetInodeFileData lee el contenido de un archivo a partir de su inodo.
// Esta función se mantiene para compatibilidad con otros comandos.
func GetInodeFileData(Inode Particiones.Inode, file *os.File, tempSuperblock Particiones.SuperBlock) (string, string) {
	var output strings.Builder
	var content strings.Builder

	// Iterar sobre los bloques del inodo
	for i := 0; i < 12; i++ {
		blockIndex := Inode.I_block[i]
		if blockIndex != -1 {
			var crrFileBlock Particiones.FileBlock
			blockPos := tempSuperblock.S_block_start + blockIndex*tempSuperblock.S_block_size
			if err := Utils.ReadFile(file, &crrFileBlock, int64(blockPos)); err == nil {
				// Limpiar bytes nulos antes de agregar al contenido
				content.WriteString(strings.TrimRight(string(crrFileBlock.B_content[:]), "\x00"))
			} else {
				output.WriteString(fmt.Sprintf("Error al leer el bloque de archivo %d: %v\n", blockIndex, err))
			}
		}
	}

	// Recortar el contenido al tamaño real especificado en el inodo
	if Inode.I_size < int32(content.Len()) {
		return content.String()[:Inode.I_size], output.String()
	}

	return content.String(), output.String()
}

// Las funciones findFreeBlock y AppendToFileBlock se mantienen sin cambios ya que no
// están directamente relacionadas con la lógica de búsqueda de rutas.
// ... (tu código para findFreeBlock y AppendToFileBlock aquí) ...

func AppendToFileBlock(inode *Particiones.Inode, newData string, file *os.File, superblock Particiones.SuperBlock) (error, string) {
	// Tu implementación actual de AppendToFileBlock
	var output strings.Builder
	output.WriteString(" ============================================================================== \n")
	output.WriteString(" ========================== AGREGAR AL BLOQUE ========================== \n")
	output.WriteString(" ============================================================================== \n")

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

	output.WriteString(" ============================================================================== \n")
	output.WriteString(" ============================== FIN AGREGAR AL BLOQUE ============================== \n")
	output.WriteString(" ============================================================================== \n")

	return nil, output.String()
}

func findFreeBlock(file *os.File, superblock Particiones.SuperBlock) (int32, string) {
	// Tu implementación actual de findFreeBlock
	var output strings.Builder
	var blockBitmap []byte = make([]byte, superblock.S_blocks_count)

	output.WriteString(" ============================================================================== \n")
	output.WriteString(" =========================== BUSCANDO BLOQUE LIBRE ============================ \n")
	output.WriteString(" ============================================================================== \n")

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
