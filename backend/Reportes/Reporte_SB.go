package Reportes

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Utils"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GenerarReporteSB genera el reporte del Superblock de la partición montada con 'id'.

func GenerarReporteSB(diskPath, path string, id string) string {
	// ====================== localizar partición montada ======================
	var mountedPartition Entornos.MountedPartition
	found := false
	for _, partitions := range Entornos.GetMountedPartitions() {
		for _, p := range partitions {
			if p.MountID == id {
				mountedPartition = p
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return fmt.Sprintf("Error: No se encontró la partición montada con ID '%s'", id)
	}

	// ====================== abrir disco desde la partición montada ======================
	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		return fmt.Sprintf("Error: No se pudo abrir el disco en la ruta: %s", mountedPartition.MountPath)
	}
	defer file.Close()

	// ====================== leer superbloque usando MountStart ======================
	var sb Particiones.SuperBlock
	if err := Utils.ReadFile(file, &sb, int64(mountedPartition.MountStart)); err != nil {
		return "Error: No se pudo leer el superbloque desde el archivo"
	}

	// ====================== generar .dot del SB ======================
	if err := GenerateSBReportFromSB(sb, path); err != nil {
		return fmt.Sprintf("Error al generar el reporte SB: %v", err)
	}

	// ====================== renderizar .dot -> .jpg ======================
	dotFile := strings.TrimSuffix(path, filepath.Ext(path)) + ".dot"
	outputJpg := strings.TrimSuffix(path, filepath.Ext(path)) + ".jpg"

	cmd := exec.Command("dot", "-Tjpg", dotFile, "-o", outputJpg)
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("Error al renderizar el archivo .dot a imagen: %v", err)
	}

	return fmt.Sprintf("Reporte SB generado:\n- Imagen: %s\n- Archivo .dot: %s", outputJpg, dotFile)
}

// GenerateSBReport conserva la firma similar a la tuya original, pero ahora solo
// resuelve el SB desde la partición montada por 'id' y delega en GenerateSBReportFromSB.
func GenerateSBReport(id string, outputPath string) error {
	// Buscar la partición montada por ID
	var mountedPartition Entornos.MountedPartition
	found := false
	for _, partitions := range Entornos.GetMountedPartitions() {
		for _, p := range partitions {
			if p.MountID == id {
				mountedPartition = p
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return fmt.Errorf("partición no encontrada")
	}

	// Abrir el archivo del disco
	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		return fmt.Errorf("no se pudo abrir el disco: %v", err)
	}
	defer file.Close()

	// Leer el Superblock directamente desde MountStart
	var sb Particiones.SuperBlock
	if err := Utils.ReadFile(file, &sb, int64(mountedPartition.MountStart)); err != nil {
		return fmt.Errorf("no se pudo leer el superbloque: %v", err)
	}

	return GenerateSBReportFromSB(sb, outputPath)
}

// GenerateSBReportFromSB escribe el .dot del SB y crea carpetas si es necesario.
func GenerateSBReportFromSB(sb Particiones.SuperBlock, outputPath string) error {
	// Si tu estructura sb guarda fechas como arrays de bytes, aquí solo "visualizamos".
	// Si quieres mostrar la fecha actual (igual que en tu código original), actualizamos:
	currentDate := time.Now().Format("02/01/2006")
	currentDate2 := time.Now().Format("02/01/2006")
	copy(sb.S_mtime[:], currentDate)
	copy(sb.S_unmtime[:], currentDate2)

	// Asegurar que exista la carpeta de salida
	reportsDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		return fmt.Errorf("error al crear la carpeta de reportes: %v", err)
	}

	// Construir ruta del .dot a partir del outputPath
	dotFilePath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dot"
	dotFile, err := os.Create(dotFilePath)
	if err != nil {
		return fmt.Errorf("error al crear el archivo .dot de reporte: %v", err)
	}
	defer dotFile.Close()

	// NOTA: si sb.S_magic es []byte/array, conviértelo correctamente.
	// Aquí asumimos que S_magic es un entero en tu struct.
	graphContent := fmt.Sprintf(`
digraph G {
    node [fillcolor=lightyellow style=filled]
    rankdir=LR;
    subgraph cluster_SB {
        color=lightblue fillcolor=aliceblue label="Superblock Report" style=filled
        sb [label=<<table border="0" cellborder="1" cellspacing="0" cellpadding="4">
            <tr><td colspan="2" bgcolor="lightblue"><b>Superblock Information</b></td></tr>
            <tr><td><b>S_filesystem_type</b></td><td>%d (EXT2)</td></tr>
            <tr><td><b>S_inodes_count</b></td><td>%d</td></tr>
            <tr><td><b>S_blocks_count</b></td><td>%d</td></tr>
            <tr><td><b>S_free_blocks_count</b></td><td>%d</td></tr>
            <tr><td><b>S_free_inodes_count</b></td><td>%d</td></tr>
            <tr><td><b>S_mtime</b></td><td>%s</td></tr>
            <tr><td><b>S_umtime</b></td><td>%s</td></tr>
            <tr><td><b>S_mnt_count</b></td><td>%d</td></tr>
            <tr><td><b>S_magic</b></td><td>0x%X</td></tr>
            <tr><td><b>S_inode_size</b></td><td>%d bytes</td></tr>
            <tr><td><b>S_block_size</b></td><td>%d bytes</td></tr>
            <tr><td><b>S_bm_inode_start</b></td><td>%d</td></tr>
            <tr><td><b>S_bm_block_start</b></td><td>%d</td></tr>
            <tr><td><b>S_inode_start</b></td><td>%d</td></tr>
            <tr><td><b>S_block_start</b></td><td>%d</td></tr>
            <tr><td><b>S_fist_ino</b></td><td>%d</td></tr>
            <tr><td><b>S_first_blo</b></td><td>%d</td></tr>
        </table>> shape=plaintext]
    }
}
`, sb.S_filesystem_type, sb.S_inodes_count, sb.S_blocks_count, sb.S_free_blocks_count,
		sb.S_free_inodes_count, currentDate, currentDate2, sb.S_mnt_count,
		sb.S_magic, sb.S_inode_size, sb.S_block_size, sb.S_bm_inode_start,
		sb.S_bm_block_start, sb.S_inode_start, sb.S_block_start, sb.S_first_ino, sb.S_first_blo)

	if _, err := dotFile.WriteString(graphContent); err != nil {
		return fmt.Errorf("error al escribir en el archivo .dot: %v", err)
	}

	return nil
}
