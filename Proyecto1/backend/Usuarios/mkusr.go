package Usuarios

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

func MKUSR(user string, pass string, grp string) string {
	var output strings.Builder
	output.WriteString(" ==================================================================== \n")
	output.WriteString(" ========================== INICIO MKUSR  =========================== \n")
	output.WriteString(fmt.Sprintf("  Usuario: %s\n", user))
	output.WriteString(fmt.Sprintf("  Contraseña: %s\n", pass))
	output.WriteString(fmt.Sprintf("  Grupo: %s\n", grp))
	output.WriteString(" ==================================================================== \n")

	// Verificar si el usuario actual es root
	if !IsRootLoggedIn() {
		return "❌ Error: Solo el usuario root puede crear usuarios."
	}

	if len(user) > 10 || len(pass) > 10 || len(grp) > 10 {
		return "❌ Error: user, pass y grp deben tener máximo 10 caracteres."
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

	// Variables para el grupo y validación de usuario
	groupID := -1
	userExists := false
	blockSize := binary.Size(Particiones.FileBlock{})

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

		content := string(fileBlock.B_content[:])
		lines := strings.Split(content, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			parts := strings.Split(line, ",")

			// Verificar si es una línea de grupo
			if len(parts) >= 4 && strings.TrimSpace(parts[3]) == "G" {
				// Formato: , 8, 8,G,usuarios
				groupName := strings.TrimSpace(parts[4])
				if groupName == grp {
					groupNum := strings.TrimSpace(parts[1])
					if id, err := strconv.Atoi(groupNum); err == nil {
						groupID = id
					}
				}
			}

			// Verificar si es una línea de usuario
			if len(parts) >= 5 && strings.TrimSpace(parts[1]) == "U" {
				userName := strings.TrimSpace(parts[3])
				if userName == user {
					userExists = true
				}
			}
		}
	}

	// Validaciones después de revisar todos los bloques
	if groupID == -1 {
		return fmt.Sprintf("❌ Error: El grupo '%s' no existe en el sistema.", grp)
	}

	if userExists {
		return fmt.Sprintf("❌ Error: El usuario '%s' ya existe en el sistema.", user)
	}

	// Crear la nueva línea del usuario
	newUserLine := fmt.Sprintf("%d,U,%s,%s,%s\n", groupID, grp, user, pass)

	// Agregar el usuario de manera persistente en users.txt
	err, log = AppendToFileBlock(&crrInode, newUserLine, file, tempSuperblock)
	output.WriteString(log)
	if err != nil {
		return fmt.Sprintf("❌ Error al escribir en el archivo users.txt: %v", err)
	}

	output.WriteString("✅ Usuario creado correctamente. \n")
	output.WriteString(" ==================================================================== \n")
	output.WriteString(" ==========================  FIN MKUSR   ============================  \n")
	output.WriteString(" ==================================================================== \n")

	return output.String()
}
