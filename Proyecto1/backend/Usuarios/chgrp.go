package Usuarios

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func CHGRP(user string, grp string) string {
	var output strings.Builder

	output.WriteString(" ======================================================================\n")
	output.WriteString(" =========================== INICIO CHGRP  =========================== \n")
	output.WriteString(" ======================================================================\n")
	output.WriteString(fmt.Sprintf("  Usuario: %s\n", user))
	output.WriteString(fmt.Sprintf("  Nuevo grupo: %s\n", grp))

	if !IsRootLoggedIn() {
		return "❌ Error: Solo el usuario root puede cambiar grupos de usuarios."
	}

	mountedPartition := Entornos.GetCurrentMountedPartition()
	if mountedPartition == nil {
		return "❌ Error: No hay ninguna partición montada."
	}

	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		return fmt.Sprintf("❌ Error al abrir el archivo: %v", err)
	}
	defer file.Close()

	var tempSuperblock Particiones.SuperBlock
	if err := Utils.ReadFile(file, &tempSuperblock, int64(mountedPartition.MountStart)); err != nil {
		return fmt.Sprintf("❌ Error al leer el Superblock: %v", err)
	}

	indexInode, log := InitSearch("/users.txt", file, tempSuperblock)
	output.WriteString(log)
	if indexInode == -1 {
		return "❌ Error: No se encontró el archivo users.txt."
	}

	var crrInode Particiones.Inode
	if err := Utils.ReadFile(file, &crrInode, int64(tempSuperblock.S_inode_start+indexInode*int32(binary.Size(Particiones.Inode{})))); err != nil {
		return fmt.Sprintf("❌ Error al leer el inodo de users.txt: %v", err)
	}

	blockSize := binary.Size(Particiones.FileBlock{})
	userFound := false
	var userLine string
	var userBlockIndex int32 = -1
	var targetGroupBlockIndex int32 = -1
	newGroupID := -1
	var userWasOnlyContent bool = false

	// Primera pasada: Buscar usuario y validar nuevo grupo
	for i, block := range crrInode.I_block {
		if block == -1 {
			continue
		}

		var fileBlock Particiones.FileBlock
		blockOffset := int64(tempSuperblock.S_block_start + block*int32(blockSize))

		if err := Utils.ReadFile(file, &fileBlock, blockOffset); err != nil {
			return fmt.Sprintf("❌ Error al leer bloque de archivo: %v", err)
		}

		content := string(fileBlock.B_content[:])
		lines := strings.Split(content, "\n")
		nonEmptyLines := 0

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			nonEmptyLines++

			parts := strings.Split(line, ",")

			// Buscar el usuario
			if len(parts) >= 5 && strings.TrimSpace(parts[1]) == "U" && strings.TrimSpace(parts[3]) == user {
				if id, err := strconv.Atoi(strings.TrimSpace(parts[0])); err != nil || id <= 0 {
					return "❌ Error: El usuario está eliminado."
				}
				userLine = line
				userBlockIndex = int32(i)
				userFound = true

				if nonEmptyLines == 1 {
					userWasOnlyContent = true
				}
			}

			// Buscar el nuevo grupo (formato: , 3, 3,G,adm)
			if len(parts) >= 5 && strings.TrimSpace(parts[3]) == "G" {
				groupName := strings.TrimSpace(parts[4])
				if groupName == grp {
					groupNum := strings.TrimSpace(parts[1])
					if id, err := strconv.Atoi(groupNum); err == nil && id > 0 {
						newGroupID = id
						targetGroupBlockIndex = int32(i)
					}
				}
			}
		}
	}

	if !userFound {
		return fmt.Sprintf("❌ Error: El usuario '%s' no existe en el sistema.", user)
	}

	if newGroupID == -1 {
		return fmt.Sprintf("❌ Error: El grupo '%s' no existe o está eliminado.", grp)
	}

	// Eliminar usuario del bloque actual
	var userBlock Particiones.FileBlock
	userBlockOffset := int64(tempSuperblock.S_block_start + crrInode.I_block[userBlockIndex]*int32(blockSize))
	if err := Utils.ReadFile(file, &userBlock, userBlockOffset); err != nil {
		return fmt.Sprintf("❌ Error al leer bloque de usuario: %v", err)
	}

	userContent := string(userBlock.B_content[:])
	userLines := strings.Split(userContent, "\n")
	var newUserLines []string
	for _, line := range userLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) >= 5 && strings.TrimSpace(parts[1]) == "U" && strings.TrimSpace(parts[3]) == user {
			continue // Eliminar esta línea
		}
		newUserLines = append(newUserLines, line)
	}

	// Si el bloque solo contenía al usuario, eliminarlo completamente
	if userWasOnlyContent {
		output.WriteString("🗑️ Eliminando bloque que solo contenía al usuario...")

		// Liberar el bloque
		if err := freeBlock(file, tempSuperblock, crrInode.I_block[userBlockIndex]); err != nil {
			return fmt.Sprintf("❌ Error al liberar bloque: %v", err)
		}

		// Desplazar los bloques posteriores
		for i := int(userBlockIndex); i < len(crrInode.I_block)-1; i++ {
			crrInode.I_block[i] = crrInode.I_block[i+1]
		}
		crrInode.I_block[len(crrInode.I_block)-1] = -1

		// Actualizar el inodo
		inodeOffset := int64(tempSuperblock.S_inode_start + indexInode*int32(binary.Size(Particiones.Inode{})))
		if err := Utils.WriteFile(file, crrInode, inodeOffset); err != nil {
			return fmt.Sprintf("❌ Error al actualizar inodo: %v", err)
		}
	} else {
		// Actualizar bloque original sin el usuario
		userBlock.B_content = [64]byte{}
		copy(userBlock.B_content[:], strings.Join(newUserLines, "\n"))
		if err := Utils.WriteFile(file, userBlock, userBlockOffset); err != nil {
			return fmt.Sprintf("❌ Error al actualizar bloque original: %v", err)
		}
	}

	// Actualizar el ID de grupo en la línea del usuario
	parts := strings.Split(userLine, ",")
	parts[0] = strconv.Itoa(newGroupID)
	parts[2] = grp
	updatedUserLine := strings.Join(parts, ",")

	// Estrategia de colocación inteligente
	var targetBlock Particiones.FileBlock
	targetBlockOffset := int64(tempSuperblock.S_block_start + crrInode.I_block[targetGroupBlockIndex]*int32(blockSize))
	if err := Utils.ReadFile(file, &targetBlock, targetBlockOffset); err != nil {
		return fmt.Sprintf("❌ Error al leer bloque destino: %v", err)
	}

	targetContent := string(targetBlock.B_content[:])
	targetLines := strings.Split(targetContent, "\n")

	if len(targetContent)+len(updatedUserLine)+1 <= 64 {
		// Agregar al bloque del grupo
		targetLines = append(targetLines, updatedUserLine)
		newTargetContent := strings.Join(targetLines, "\n")

		targetBlock.B_content = [64]byte{}
		copy(targetBlock.B_content[:], newTargetContent)
		if err := Utils.WriteFile(file, targetBlock, targetBlockOffset); err != nil {
			return fmt.Sprintf("❌ Error al actualizar bloque destino: %v", err)
		}
		output.WriteString("✅ Usuario agregado al bloque del grupo destino.")
	} else {
		// Crear nuevo bloque contiguo al del grupo
		output.WriteString("⚠️ No hay espacio en bloque actual, creando nuevo bloque...")

		// Buscar un bloque libre
		newBlockIndex, log := findFreeBlock(file, tempSuperblock)
		output.WriteString(log)
		if newBlockIndex == -1 {
			return "❌ Error: No hay bloques libres disponibles."
		}

		// Buscar la posición para insertar el nuevo bloque (después del bloque del grupo)
		insertPosition := -1
		for i := 0; i < len(crrInode.I_block); i++ {
			if crrInode.I_block[i] == crrInode.I_block[targetGroupBlockIndex] {
				insertPosition = i
				break
			}
		}

		if insertPosition == -1 {
			return "❌ Error: No se pudo determinar posición para nuevo bloque."
		}

		// Desplazar bloques posteriores para hacer espacio
		for i := len(crrInode.I_block) - 1; i > insertPosition+1; i-- {
			crrInode.I_block[i] = crrInode.I_block[i-1]
		}

		// Asignar nuevo bloque
		crrInode.I_block[insertPosition+1] = newBlockIndex

		// Actualizar el inodo
		inodeOffset := int64(tempSuperblock.S_inode_start + indexInode*int32(binary.Size(Particiones.Inode{})))
		if err := Utils.WriteFile(file, crrInode, inodeOffset); err != nil {
			return fmt.Sprintf("❌ Error al actualizar inodo: %v", err)
		}

		// Crear nuevo bloque con el usuario
		var newBlock Particiones.FileBlock
		newBlock.B_content = [64]byte{}
		copy(newBlock.B_content[:], updatedUserLine)

		newBlockOffset := int64(tempSuperblock.S_block_start + newBlockIndex*int32(blockSize))
		if err := Utils.WriteFile(file, newBlock, newBlockOffset); err != nil {
			return fmt.Sprintf("❌ Error al escribir nuevo bloque: %v", err)
		}

		output.WriteString("✅ Nuevo bloque creado para usuario cambiado de grupo.")
	}

	output.WriteString("✅ Usuario movido al nuevo grupo correctamente.")

	output.WriteString(" \n ===================================================================== \n")
	output.WriteString(" ===========================    FIN CHGRP  =========================== \n")
	output.WriteString(" ===================================================================== \n")

	return output.String()
}

func freeBlock(file *os.File, sb Particiones.SuperBlock, blockIndex int32) error {
	// Leer el bitmap de bloques
	bitmapSize := sb.S_blocks_count
	bitmap := make([]byte, bitmapSize)
	if _, err := file.Seek(int64(sb.S_bm_block_start), 0); err != nil {
		return err
	}
	if err := binary.Read(file, binary.LittleEndian, bitmap); err != nil {
		return err
	}

	// Marcar el bloque como libre
	if blockIndex >= 0 && blockIndex < int32(bitmapSize) {
		bitmap[blockIndex] = 0
	} else {
		return fmt.Errorf(" índice de bloque inválido")
	}

	// Escribir el bitmap actualizado
	if _, err := file.Seek(int64(sb.S_bm_block_start), 0); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, bitmap); err != nil {
		return err
	}

	return nil
}
