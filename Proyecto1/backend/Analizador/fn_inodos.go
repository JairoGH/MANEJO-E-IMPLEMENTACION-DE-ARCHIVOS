// Analizador/fn_inodos.go
package Analizador

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

func fn_inodos(params string) string {
	// Parseo de flags
	pm := ExtractParams(params)
	id := strings.TrimSpace(pm["id"])
	if id == "" {
		return "Error: debes proporcionar -id=<ID_MONTADO>\n"
	}

	// Buscar la partición montada por ID
	var part Entornos.MountedPartition
	found := false
	for _, lst := range Entornos.GetMountedPartitions() {
		for _, p := range lst {
			if p.MountID == id {
				part = p
				found = true
				break
			}
		}
	}
	if !found {
		return fmt.Sprintf("Error: no se encontró una partición montada con id %s\n", id)
	}

	// Abrir el archivo de disco
	f, err := Utils.OpenFile(part.MountPath)
	if err != nil {
		return fmt.Sprintf("Error abriendo disco en %s: %v\n", part.MountPath, err)
	}
	defer f.Close()

	// Leer superbloque (se asume en MountStart)
	var sb Particiones.SuperBlock
	if err := Utils.ReadFile(f, &sb, int64(part.MountStart)); err != nil {
		return fmt.Sprintf("Error leyendo SuperBlock: %v\n", err)
	}
	if sb.S_inodes_count <= 0 {
		return fmt.Sprintf("SuperBlock inválido: S_inodes_count=%d\n", sb.S_inodes_count)
	}

	// Asegurar tamaño de inodo si no está seteado
	inodeSize := sb.S_inode_size
	if inodeSize <= 0 {
		inodeSize = int32(binary.Size(Particiones.Inode{}))
	}

	// Contar inodos usados con el bitmap de inodos
	used := int32(0)
	if u, err := countUsedInodes(f, sb, part); err == nil {
		used = u
	} else {
		// Si falla el bitmap, al menos no rompemos el comando
		return fmt.Sprintf("Error leyendo bitmap de inodos: %v\n", err)
	}
	free := sb.S_inodes_count - used

	// Datos útiles para depurar offsets
	inodeTblStartAbs := part.MountStart + sb.S_inode_start
	bmInodeStartAbs := part.MountStart + sb.S_bm_inode_start

	out := &strings.Builder{}
	fmt.Fprintf(out, "===== REPORTE DE INODOS =====\n")
	fmt.Fprintf(out, "ID montado     : %s\n", id)
	fmt.Fprintf(out, "Ruta de disco  : %s\n", part.MountPath)
	fmt.Fprintf(out, "Total inodos   : %d\n", sb.S_inodes_count)
	fmt.Fprintf(out, "Inodos usados  : %d\n", used)
	fmt.Fprintf(out, "Inodos libres  : %d\n", free)
	fmt.Fprintf(out, "Tamaño inodo   : %d bytes\n", inodeSize)
	fmt.Fprintf(out, "Inode table @  : %d (abs)\n", inodeTblStartAbs)
	fmt.Fprintf(out, "Bm inodos   @  : %d (abs)\n", bmInodeStartAbs)

	// Opcional: listar los primeros N inodos usados (útil para validar reportes)
	if strings.ToLower(pm["listar"]) == "true" {
		indices, err := firstUsedInodes(f, sb, part, 32) // muestra hasta 32
		if err == nil && len(indices) > 0 {
			fmt.Fprintf(out, "Primeros inodos usados: %v\n", indices)
		}
	}

	return out.String()
}

// Lee el bitmap de inodos y devuelve cuántos bits están en 1.
func countUsedInodes(file *os.File, sb Particiones.SuperBlock, part Entornos.MountedPartition) (int32, error) {
	var count int32
	bmStart := part.MountStart + sb.S_bm_inode_start
	var b [1]byte
	for i := int32(0); i < sb.S_inodes_count; i++ {
		byteOff := bmStart + (i / 8)
		bitMask := byte(1 << (i % 8))
		if err := Utils.ReadFile(file, &b, int64(byteOff)); err != nil {
			return 0, err
		}
		if (b[0] & bitMask) != 0 {
			count++
		}
	}
	return count, nil
}

// Devuelve los primeros n índices de inodos usados (para verificación rápida)
func firstUsedInodes(file *os.File, sb Particiones.SuperBlock, part Entornos.MountedPartition, n int) ([]int32, error) {
	var indices []int32
	bmStart := part.MountStart + sb.S_bm_inode_start
	var b [1]byte
	for i := int32(0); i < sb.S_inodes_count; i++ {
		byteOff := bmStart + (i / 8)
		bitMask := byte(1 << (i % 8))
		if err := Utils.ReadFile(file, &b, int64(byteOff)); err != nil {
			return nil, err
		}
		if (b[0] & bitMask) != 0 {
			indices = append(indices, i)
			if len(indices) >= n {
				break
			}
		}
	}
	return indices, nil
}
