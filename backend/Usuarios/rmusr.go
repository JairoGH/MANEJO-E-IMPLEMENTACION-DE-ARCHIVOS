package Usuarios

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"strings"
)

func RMUSR(user string) string {
	var output strings.Builder
	output.WriteString(" ==================================================================== \n")
	output.WriteString(" ========================== INICIO RMUSR  =========================== \n")
	output.WriteString(fmt.Sprintf("  Usuario a eliminar: %s\n", user))
	output.WriteString(" ==================================================================== \n")

	// Verificar si el usuario actual es root
	if !IsRootLoggedIn() {
		return "❌ Error: Solo el usuario root puede eliminar usuarios."
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
	userFound := false

	// Recorrer todos los bloques del inodo
	for _, block := range crrInode.I_block {
		if block == -1 {
			continue
		}

		var fileBlock Particiones.FileBlock
		blockOffset := int64(tempSuperblock.S_block_start + block*int32(blockSize))

		if err := Utils.ReadFile(file, &fileBlock, blockOffset); err != nil {
			return fmt.Sprintf("❌ Error al leer bloque de archivo: %v", err)
		}

		// Convertir el contenido a string y dividir por líneas
		content := string(fileBlock.B_content[:])
		lines := strings.Split(content, "\n")
		modified := false

		// Buscar el usuario en este bloque
		for i, line := range lines {
			if line == "" {
				continue
			}

			parts := strings.Split(line, ",")
			if len(parts) >= 5 && strings.TrimSpace(parts[1]) == "U" && strings.TrimSpace(parts[3]) == user {
				output.WriteString("✅ Usuario encontrado, marcando como eliminado en su bloque.\n")
				// Marcar el usuario como eliminado (ID = 0)
				lines[i] = "0,U," + strings.TrimSpace(parts[2]) + "," + strings.TrimSpace(parts[3]) + "," + strings.TrimSpace(parts[4])
				modified = true
				userFound = true
				break
			}
		}

		// Si encontramos y modificamos el usuario, actualizar el bloque
		if modified {
			// Unir las líneas y asegurarse de que no exceda el tamaño del bloque
			newContent := strings.Join(lines, "\n")
			if len(newContent) > len(fileBlock.B_content) {
				newContent = newContent[:len(fileBlock.B_content)]
			}

			// Limpiar el bloque y copiar el nuevo contenido
			fileBlock.B_content = [64]byte{}
			copy(fileBlock.B_content[:], newContent)

			// Escribir el bloque modificado en el archivo
			if err := Utils.WriteFile(file, fileBlock, blockOffset); err != nil {
				return fmt.Sprintf("❌ Error al escribir el bloque actualizado: %v", err)
			}

			output.WriteString("✅ Usuario eliminado correctamente. \n")
			break
		}
	}

	if !userFound {
		output.WriteString("❌ Error: El usuario no existe en el sistema. \n")
	}

	output.WriteString(" ==================================================================== \n")
	output.WriteString(" ==========================  FIN RMUSR   ============================  \n")
	output.WriteString(" ==================================================================== \n")
	return output.String()
}
