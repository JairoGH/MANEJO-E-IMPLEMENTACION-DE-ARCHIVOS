package Usuarios

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"strings"
)

func RMGRP(name string) string {
	var output strings.Builder
	output.WriteString(" ============================================================ \n")
	output.WriteString(" ==================== INICIANDO RMGRP  =====================  \n")
	output.WriteString(" ============================================================ \n")
	output.WriteString(fmt.Sprintf("  Nombre del grupo a eliminar: %s\n", name))

	// Verificar si el usuario actual es root
	if !IsRootLoggedIn() {
		return "❌ Error: Solo el usuario root puede eliminar grupos."
	}

	// Obtener la partición montada actualmente
	mountedPartition := Entornos.GetCurrentMountedPartition()
	if mountedPartition == nil {
		return "❌ Error: No hay ninguna partición montada."
	}

	// Abrir el archivo del sistema de archivos
	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		return fmt.Sprintf("❌ Error al abrir el archivo: %v", err)
	}
	defer file.Close()

	// Leer el Superblock de la partición
	var tempSuperblock Particiones.SuperBlock
	if err := Utils.ReadFile(file, &tempSuperblock, int64(mountedPartition.MountStart)); err != nil {
		return fmt.Sprintf("❌ Error al leer el Superblock: %v", err)
	}

	// Buscar el archivo "users.txt"
	indexInode, log := InitSearch("/users.txt", file, tempSuperblock)
	output.WriteString(log) // Agregar el log de InitSearch al log principal
	if indexInode == -1 {
		return "❌ Error: No se encontró el archivo users.txt."
	}

	// Leer el inodo del archivo "users.txt"
	var crrInode Particiones.Inode
	if err := Utils.ReadFile(file, &crrInode, int64(tempSuperblock.S_inode_start+indexInode*int32(binary.Size(Particiones.Inode{})))); err != nil {
		return fmt.Sprintf("❌ Error al leer el inodo de users.txt: %v", err)
	}

	blockSize := binary.Size(Particiones.FileBlock{})
	for _, block := range crrInode.I_block {
		if block == -1 {
			continue
		}

		var fileBlock Particiones.FileBlock
		blockOffset := int64(tempSuperblock.S_block_start + block*int32(blockSize))

		if err := Utils.ReadFile(file, &fileBlock, blockOffset); err != nil {
			return fmt.Sprintf("❌ Error al leer bloque de archivo: %v", err)
		}

		lines := strings.Split(string(fileBlock.B_content[:]), "\n")
		for i, line := range lines {
			parts := strings.Split(line, ",")
			if len(parts) >= 3 && strings.TrimSpace(parts[1]) == "G" && strings.TrimSpace(parts[2]) == name {
				output.WriteString("✅ Grupo encontrado, marcando como eliminado en su bloque. \n")
				lines[i] = "0,G," + parts[2]
				fileBlock.B_content = [64]byte{}
				copy(fileBlock.B_content[:], strings.Join(lines, "\n"))

				// Escribir el bloque actualizado en el archivo
				if err := Utils.WriteFile(file, fileBlock, blockOffset); err != nil {
					return fmt.Sprintf("❌ Error al escribir el bloque actualizado: %v", err)
				}

				output.WriteString("✅ Grupo eliminado correctamente en su bloque. \n")
				output.WriteString(" ============================================================ \n")
				output.WriteString("======================   FIN RMGRP   ====================== \n")
				output.WriteString(" ============================================================ \n")
				return output.String()
			}
		}
	}

	output.WriteString("❌ Error: El grupo no existe en el sistema.\n")
	output.WriteString("======================   FINALIZANDO RMGRP   ====================== \n")
	return output.String()
}
