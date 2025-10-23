package Usuarios

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
)

// Variable global para el contador de grupos
var (
	groupCounter int = 2
	counterMutex sync.Mutex
)

func MKGRP(name string) string {
	var output strings.Builder
	output.WriteString(" ==================================================================== \n")
	output.WriteString(" ======================== INICIO MKGRP  ======================== \n")
	output.WriteString(" ==================================================================== \n")

	output.WriteString(fmt.Sprintf("  Nombre del grupo: %s\n", name))

	// Verificar si el usuario actual es root
	if !IsRootLoggedIn() {
		return "Error: Solo el usuario root puede crear grupos."
	}

	// Obtener la partición montada actualmente
	mountedPartition := Entornos.GetCurrentMountedPartition()
	if mountedPartition == nil {
		return "Error: No hay ninguna partición montada."
	}

	// Abrir el archivo del sistema de archivos
	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		return fmt.Sprintf("Error al abrir el archivo: %v", err)
	}
	defer file.Close()

	// Leer el Superblock de la partición
	var tempSuperblock Particiones.SuperBlock
	if err := Utils.ReadFile(file, &tempSuperblock, int64(mountedPartition.MountStart)); err != nil {
		return fmt.Sprintf("Error al leer el Superblock: %v", err)
	}

	// Buscar el archivo "users.txt" en el sistema de archivos
	indexInode, log := InitSearch("/users.txt", file, tempSuperblock)
	output.WriteString(log)
	if indexInode == -1 {
		return "Error: No se encontró el archivo users.txt."
	}

	// Leer el inodo del archivo "users.txt"
	var crrInode Particiones.Inode
	if err := Utils.ReadFile(file, &crrInode, int64(tempSuperblock.S_inode_start+indexInode*int32(binary.Size(Particiones.Inode{})))); err != nil {
		return fmt.Sprintf("Error al leer el inodo de users.txt: %v", err)
	}

	// Obtener el contenido actual del archivo "users.txt"
	data, _ := GetInodeFileData(crrInode, file, tempSuperblock)

	// Verificar si el grupo ya existe
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		if strings.Contains(line, ",G,") {
			parts := strings.Split(line, ",")
			if len(parts) >= 4 && strings.TrimSpace(parts[3]) == name {
				return "Error: El grupo ya existe."
			}
		}
	}

	// Obtener y aumentar el contador de grupos de manera segura
	counterMutex.Lock()
	currentID := groupCounter
	groupCounter++
	counterMutex.Unlock()

	// Crear la línea del nuevo grupo
	newGroupLine := fmt.Sprintf(", %d, %d,G,%s\n", currentID, currentID, name)

	// Buscar un bloque libre
	newBlockIndex, log := findFreeBlock(file, tempSuperblock)
	output.WriteString(log)
	if newBlockIndex == -1 {
		return "❌ Error: No hay bloques libres disponibles."
	}

	// Escribir el nuevo grupo en el bloque
	var fileBlock Particiones.FileBlock
	copy(fileBlock.B_content[:], newGroupLine)
	blockOffset := int64(tempSuperblock.S_block_start + newBlockIndex*int32(binary.Size(Particiones.FileBlock{})))
	if err := Utils.WriteFile(file, fileBlock, blockOffset); err != nil {
		return fmt.Sprintf("❌ Error al escribir el bloque de archivo: %v", err)
	}

	// Asignar el nuevo bloque al inodo
	for i := 0; i < len(crrInode.I_block); i++ {
		if crrInode.I_block[i] == -1 {
			crrInode.I_block[i] = int32(newBlockIndex)
			break
		}
	}

	// Actualizar el tamaño del inodo
	crrInode.I_size += int32(len(newGroupLine))

	// Escribir el inodo actualizado en el archivo
	inodeOffset := int64(tempSuperblock.S_inode_start + indexInode*int32(binary.Size(Particiones.Inode{})))
	if err := Utils.WriteFile(file, crrInode, inodeOffset); err != nil {
		return fmt.Sprintf("❌ Error al actualizar el inodo: %v", err)
	}

	output.WriteString(fmt.Sprintf("✅ Grupo '%s' creado con ID %d. \n", name, currentID))
	output.WriteString(" ==================================================================== \n")
	output.WriteString(" ========================  FINALIZANDO MKGRP  ======================== \n")
	output.WriteString(" ==================================================================== \n")
	return output.String()
}
