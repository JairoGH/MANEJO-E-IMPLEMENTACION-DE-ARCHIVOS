package Estructuras

import (
	"backend/Particiones"
	"backend/Utils"
	"fmt"
	"os"
	"strings"
)

// Función auxiliar para marcar los inodos y bloques usados
func markUsedInodesAndBlocks(newSuperblock Particiones.SuperBlock, file *os.File) error {
	// Escribe el byte '1' en la posición 0 del bitmap de inodos
	if err := Utils.WriteFile(file, byte(1), int64(newSuperblock.S_bm_inode_start+0)); err != nil {
		return err
	}
	// Escribe el byte '1' en la posición 1 del bitmap de inodos
	if err := Utils.WriteFile(file, byte(1), int64(newSuperblock.S_bm_inode_start+1)); err != nil {
		return err
	}
	// Escribe el byte '1' en la posición 0 del bitmap de bloques
	if err := Utils.WriteFile(file, byte(1), int64(newSuperblock.S_bm_block_start+0)); err != nil {
		return err
	}
	// Escribe el byte '1' en la posición 1 del bitmap de bloques
	if err := Utils.WriteFile(file, byte(1), int64(newSuperblock.S_bm_block_start+1)); err != nil {
		return err
	}
	return nil
}

// Función para imprimir el Superblock (sin cambios)
func PrintSuperblock(sb Particiones.SuperBlock) string {
	// ... (El contenido de esta función se mantiene igual)
	var output strings.Builder
	output.WriteString(" ============================================================ \n")
	output.WriteString(" ======================= SUPERBLOCK ========================= \n")
	output.WriteString(" ============================================================ \n")
	output.WriteString(fmt.Sprintf(" S_filesystem_type: %d\n", sb.S_filesystem_type))
	output.WriteString(fmt.Sprintf(" S_inodes_count: %d\n", sb.S_inodes_count))
	output.WriteString(fmt.Sprintf(" S_blocks_count: %d\n", sb.S_blocks_count))
	output.WriteString(fmt.Sprintf(" S_free_blocks_count: %d\n", sb.S_free_blocks_count))
	output.WriteString(fmt.Sprintf(" S_free_inodes_count: %d\n", sb.S_free_inodes_count))
	output.WriteString(fmt.Sprintf(" S_mtime: %s\n", string(sb.S_mtime[:])))
	output.WriteString(fmt.Sprintf(" S_unmtime: %s\n", string(sb.S_unmtime[:])))
	output.WriteString(fmt.Sprintf(" S_mnt_count: %d\n", sb.S_mnt_count))
	output.WriteString(fmt.Sprintf(" S_magic: 0x%X\n", sb.S_magic))
	output.WriteString(fmt.Sprintf(" S_inode_size: %d\n", sb.S_inode_size))
	output.WriteString(fmt.Sprintf(" S_block_size: %d\n", sb.S_block_size))
	output.WriteString(fmt.Sprintf(" S_first_ino: %d\n", sb.S_first_ino))
	output.WriteString(fmt.Sprintf(" S_first_blo: %d\n", sb.S_first_blo))
	output.WriteString(fmt.Sprintf(" S_bm_inode_start: %d\n", sb.S_bm_inode_start))
	output.WriteString(fmt.Sprintf(" S_bm_block_start: %d\n", sb.S_bm_block_start))
	output.WriteString(fmt.Sprintf(" S_inode_start: %d\n", sb.S_inode_start))
	output.WriteString(fmt.Sprintf(" S_block_start: %d\n", sb.S_block_start))
	output.WriteString(" ============================================================ \n")
	return output.String()
}
