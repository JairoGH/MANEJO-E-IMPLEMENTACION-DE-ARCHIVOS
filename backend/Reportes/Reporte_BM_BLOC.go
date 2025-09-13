package Reportes

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Utils"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerarReporteBitmapBloques crea un archivo .txt con el bitmap de bloques (20 valores por línea)
// y genera además un archivo *.debug.txt con conteos y verificaciones para validar la lectura.
func GenerarReporteBitmapBloques(outputPath string, id string) string {
	// ====================== Validaciones básicas ======================
	if strings.TrimSpace(outputPath) == "" {
		return "Error: No se especificó la ruta del archivo de reporte."
	}
	outputPath = ensureTXTPath(filepath.Clean(outputPath))

	// Asegurar carpeta destino
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Sprintf("Error: No se pudo crear la carpeta destino: %v", err)
	}

	// ====================== Obtener partición montada por ID ======================
	var filePath string
	var partitionFound bool
	for _, partitions := range Entornos.GetMountedPartitions() {
		for _, partition := range partitions {
			if partition.MountID == id {
				filePath = partition.MountPath
				partitionFound = true
				break
			}
		}
		if partitionFound {
			break
		}
	}
	if !partitionFound {
		return fmt.Sprintf("Error: No se encontró la partición con ID: %s", id)
	}

	// ====================== Abrir disco ======================
	file, err := Utils.OpenFile(filePath)
	if err != nil {
		return fmt.Sprintf("Error: No se pudo abrir el archivo en la ruta: %s", filePath)
	}
	defer file.Close()

	// ====================== Leer MBR ======================
	var mbr Particiones.MBR
	if err := Utils.ReadFile(file, &mbr, 0); err != nil {
		return "Error: No se pudo leer el MBR desde el archivo."
	}

	// ====================== Localizar partición activa por ID ======================
	index := -1
	for i := 0; i < 4; i++ {
		if mbr.MBR_Partition[i].Part_Size != 0 {
			partID := strings.TrimSpace(string(mbr.MBR_Partition[i].Part_ID[:]))
			if partID == id { // comparación exacta para evitar falsos positivos
				if mbr.MBR_Partition[i].Part_Status[0] == '1' {
					index = i
				} else {
					return "Error: La partición no está montada."
				}
				break
			}
		}
	}
	if index == -1 {
		return "Error: No se encontró la partición."
	}

	// ====================== Leer Superbloque ======================
	var sb Particiones.SuperBlock
	superblockStart := mbr.MBR_Partition[index].Part_Start
	if err := Utils.ReadFile(file, &sb, int64(superblockStart)); err != nil {
		return "Error: No se pudo leer el superbloque desde el archivo."
	}

	// ====================== Validaciones del SB ======================
	if sb.S_blocks_count <= 0 {
		return "Error: El número de bloques en el superbloque es inválido."
	}
	if sb.S_bm_block_start <= 0 {
		return "Error: El puntero S_bm_block_start del superbloque es inválido."
	}

	// ====================== Leer Bitmap de Bloques ======================
	blockBitmapSize := int(sb.S_blocks_count) // 1 byte por bloque
	bitmap := make([]byte, blockBitmapSize)
	if err := Utils.ReadFile(file, &bitmap, int64(sb.S_bm_block_start)); err != nil {
		return "Error: No se pudo leer el bitmap de bloques."
	}

	// ====================== Generar contenido (20 valores por línea) ======================
	var builder strings.Builder
	for i := 0; i < len(bitmap); i++ {
		if byteIsUsed(bitmap[i]) {
			builder.WriteString("1")
		} else {
			builder.WriteString("0")
		}
		if (i+1)%20 == 0 {
			builder.WriteString("\n")
		} else {
			builder.WriteString(" ")
		}
	}
	if len(bitmap)%20 != 0 {
		builder.WriteString("\n")
	}

	// ====================== Escribir archivo local (bitmap plano) ======================
	reportFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Sprintf("Error: No se pudo crear el archivo de reporte en: %s", outputPath)
	}
	defer reportFile.Close()
	if _, err := reportFile.WriteString(builder.String()); err != nil {
		return "Error: No se pudo escribir en el archivo de reporte."
	}
	_ = reportFile.Sync()

	// ====================== DEBUG: verificación y resumen ======================
	debugPath := deriveDebugPath(outputPath)
	if err := writeDebugBMBlock(debugPath, filePath, id, &mbr, &sb, bitmap); err != nil {
		// No detenemos el flujo por el debug: solo informamos en el mensaje final.
		return fmt.Sprintf(
			"Reporte bitmap de bloques generado correctamente:\n- Archivo: %s\n- Advertencia: no se pudo crear el debug: %v",
			outputPath, err,
		)
	}

	// ====================== Resultado ======================
	return fmt.Sprintf("Reporte bitmap de bloques generado correctamente:\n- Archivo: %s\n- Debug: %s", outputPath, debugPath)
}

// ------------------------- Helpers -------------------------

// Mapea el byte del bitmap a booleano "usado".
// Acepta 0 y '0' como libre; 1 y '1' como usado. Cualquier otro valor ≠ 0 se toma como usado.
func byteIsUsed(b byte) bool {
	if b == 0 || b == '0' {
		return false
	}
	if b == 1 || b == '1' {
		return true
	}
	// valor no estándar: tratamos cualquier ≠ 0 como usado
	return true
}

// Genera un archivo *.debug.txt con métricas y primeras posiciones utilizadas.
func writeDebugBMBlock(debugPath, diskPath, id string, mbr *Particiones.MBR, sb *Particiones.SuperBlock, bitmap []byte) error {
	if err := os.MkdirAll(filepath.Dir(debugPath), 0o755); err != nil {
		return err
	}

	var used, free, nonStd int
	usedIdx := make([]int, 0, 256) // guardamos hasta 256 índices para inspección rápida

	for i, b := range bitmap {
		switch b {
		case 0, '0':
			free++
		case 1, '1':
			used++
			if len(usedIdx) < cap(usedIdx) {
				usedIdx = append(usedIdx, i)
			}
		default:
			// no estándar: ≠ 0 y ≠ 1. Lo contamos y tratamos como usado.
			nonStd++
			used++
			if len(usedIdx) < cap(usedIdx) {
				usedIdx = append(usedIdx, i)
			}
		}
	}

	total := len(bitmap)
	percent := 0.0
	if total > 0 {
		percent = 100.0 * float64(used) / float64(total)
	}

	var dbg strings.Builder
	dbg.WriteString("========== DEBUG BM_BLOCK ==========\n")
	dbg.WriteString(fmt.Sprintf("Disco          : %s\n", diskPath))
	dbg.WriteString(fmt.Sprintf("ID montado     : %s\n", id))
	dbg.WriteString("\n-- Superbloque --\n")
	dbg.WriteString(fmt.Sprintf("S_inodes_count     : %d\n", sb.S_inodes_count))
	dbg.WriteString(fmt.Sprintf("S_blocks_count     : %d\n", sb.S_blocks_count))
	dbg.WriteString(fmt.Sprintf("S_bm_inode_start   : %d (abs)\n", sb.S_bm_inode_start))
	dbg.WriteString(fmt.Sprintf("S_bm_block_start   : %d (abs)\n", sb.S_bm_block_start))
	dbg.WriteString(fmt.Sprintf("S_inode_start      : %d (abs)\n", sb.S_inode_start))
	dbg.WriteString(fmt.Sprintf("S_block_start      : %d (abs)\n", sb.S_block_start))
	dbg.WriteString(fmt.Sprintf("S_inode_size       : %d bytes\n", sb.S_inode_size))
	dbg.WriteString(fmt.Sprintf("S_block_size       : %d bytes\n", sb.S_block_size))

	dbg.WriteString("\n-- Bitmap bloques --\n")
	dbg.WriteString(fmt.Sprintf("Total entradas (bytes) : %d\n", total))
	dbg.WriteString(fmt.Sprintf("Usados (1)             : %d\n", used))
	dbg.WriteString(fmt.Sprintf("Libres (0)             : %d\n", free))
	dbg.WriteString(fmt.Sprintf("No estándar (≠0/1)     : %d\n", nonStd))
	dbg.WriteString(fmt.Sprintf("Ocupación              : %.2f%%\n", percent))

	dbg.WriteString("\nPrimeros índices en 1 (hasta 256):\n")
	if len(usedIdx) == 0 {
		dbg.WriteString("  (ninguno)\n")
	} else {
		dbg.WriteString("  ")
		for i, idx := range usedIdx {
			if i > 0 {
				dbg.WriteString(", ")
			}
			dbg.WriteString(fmt.Sprintf("%d", idx))
			// Salto de línea cada 20 índices para comodidad
			if (i+1)%20 == 0 {
				dbg.WriteString("\n  ")
			}
		}
		dbg.WriteString("\n")
	}

	// Guardar archivo
	f, err := os.Create(debugPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(dbg.String()); err != nil {
		return err
	}
	_ = f.Sync()
	return nil
}

// ensureTXTPath garantiza que la ruta termine en .txt
func ensureTXTPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return path + ".txt"
	}
	if ext != ".txt" {
		return strings.TrimSuffix(path, filepath.Ext(path)) + ".txt"
	}
	return path
}

// deriveDebugPath genera el nombre de archivo *.debug.txt a partir del output principal.
func deriveDebugPath(path string) string {
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return base + ".debug.txt"
}
