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
)

// GenerarReporteBloques crea un reporte visual que agrupa y relaciona
// todos los bloques utilizados en la partición.
func GenerarReporteBloques(pathFileLs string, outputPath string, id string) string {
	// --- 1. Validaciones y Obtención de Partición ---
	if strings.TrimSpace(outputPath) == "" {
		return "Error: Debe indicar una ruta de salida con el parámetro -path."
	}

	mp, found := Entornos.GetMountedPartitionByID(id)
	if !found {
		return fmt.Sprintf("Error: No se encontró la partición montada con ID %s", id)
	}

	// --- 2. Abrir Disco y Leer Superbloque ---
	file, err := Utils.OpenFile(mp.MountPath)
	if err != nil {
		return fmt.Sprintf("Error abriendo el disco: %v", err)
	}
	defer file.Close()

	var sb Particiones.SuperBlock
	if err := Utils.ReadFile(file, &sb, int64(mp.MountStart)); err != nil {
		return fmt.Sprintf("Error leyendo el Superbloque: %v", err)
	}

	// --- 3. Leer el Bitmap de Inodos ---
	inodeBitmapSize := (sb.S_inodes_count + 7) / 8
	inodeBitmap := make([]byte, inodeBitmapSize)
	if err := Utils.ReadFile(file, &inodeBitmap, int64(sb.S_bm_inode_start)); err != nil {
		return fmt.Sprintf("Error al leer el bitmap de inodos: %v", err)
	}

	// --- 4. Construir el Archivo DOT ---
	var dot strings.Builder
	dot.WriteString("digraph AllBlocks {\n")
	dot.WriteString("  rankdir=LR;\n")
	dot.WriteString("  node [shape=plaintext];\n")
	dot.WriteString("  label=\"Reporte de Bloques Utilizados (Agrupados por Inodo)\";\n\n")

	processedBlocks := make(map[int32]bool)

	// Iterar sobre todos los posibles inodos
	for i := int32(0); i < sb.S_inodes_count; i++ {
		// Verificar si el inodo 'i' está en uso
		byteIndex := i / 8
		bitIndex := i % 8
		if (inodeBitmap[byteIndex] & (1 << bitIndex)) == 0 {
			continue // Si el inodo no está en uso, saltarlo
		}

		// Leer el inodo correspondiente
		var ino Particiones.Inode
		inodePos := sb.S_inode_start + i*sb.S_inode_size
		if err := Utils.ReadFile(file, &ino, int64(inodePos)); err != nil {
			continue
		}

		isDir := ino.I_type[0] == '0'
		nodeType := "Archivo"
		if isDir {
			nodeType = "Directorio"
		}

		// Iniciar un subgráfico para agrupar los bloques de este inodo
		dot.WriteString(fmt.Sprintf("  subgraph cluster_inode_%d {\n", i))
		dot.WriteString(fmt.Sprintf("    label=\"Bloques del Inodo %d (%s)\";\n", i, nodeType))
		dot.WriteString("    style=filled;\n    color=lightgrey;\n")

		var prevNodeID string
		// Iterar sobre los bloques del inodo para dibujarlos y conectarlos
		for j := 0; j < 12; j++ {
			blockIndex := ino.I_block[j]
			if blockIndex == -1 {
				continue
			}

			nodeID := fmt.Sprintf("block_%d", blockIndex)

			// Solo dibujar el nodo si no ha sido procesado antes
			if !processedBlocks[blockIndex] {
				if isDir {
					var folderBlock Particiones.FolderBlock
					blockPos := sb.S_block_start + blockIndex*sb.S_block_size
					if err := Utils.ReadFile(file, &folderBlock, int64(blockPos)); err == nil {
						dot.WriteString(renderFolderBlockNode(nodeID, blockIndex, &folderBlock))
					}
				} else { // Es un archivo
					var fileBlock Particiones.FileBlock
					blockPos := sb.S_block_start + blockIndex*sb.S_block_size
					if err := Utils.ReadFile(file, &fileBlock, int64(blockPos)); err == nil {
						dot.WriteString(renderFileBlockNode(nodeID, blockIndex, &fileBlock))
					}
				}
				processedBlocks[blockIndex] = true
			}

			// Conectar con el bloque anterior de la misma cadena
			if prevNodeID != "" {
				dot.WriteString(fmt.Sprintf("    %s -> %s;\n", prevNodeID, nodeID))
			}
			prevNodeID = nodeID
		}
		dot.WriteString("  }\n\n") // Cerrar el subgráfico
	}

	dot.WriteString("}\n")

	// --- 5. Guardar .dot y Generar Imagen ---
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Sprintf("Error creando la carpeta de salida: %v", err)
	}
	dotPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dot"
	if err := os.WriteFile(dotPath, []byte(dot.String()), 0644); err != nil {
		return fmt.Sprintf("Error escribiendo el archivo DOT: %v", err)
	}

	cmd := exec.Command("dot", "-Tjpg", dotPath, "-o", outputPath)
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("Error generando la imagen con Graphviz: %v\nAsegúrate de que Graphviz esté instalado y en el PATH del sistema.", err)
	}

	return fmt.Sprintf("Reporte de bloques relacionado generado exitosamente en: %s", outputPath)
}

// renderFolderBlockNode crea el código DOT para un bloque de tipo Carpeta.
func renderFolderBlockNode(nodeID string, blockNumber int32, blk *Particiones.FolderBlock) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("    %s [label=<\n", nodeID)) // Indentado para el subgráfico
	b.WriteString(`    <TABLE BORDER="1" CELLBORDER="1" CELLSPACING="0" BGCOLOR="#C1FFC1">`)
	b.WriteString(fmt.Sprintf(`<TR><TD COLSPAN="2" BGCOLOR="#2ECC71"><B>Bloque Carpeta %d</B></TD></TR>`, blockNumber))
	b.WriteString(`<TR><TD><B>Nombre</B></TD><TD><B>Inodo</B></TD></TR>`)

	for _, c := range blk.B_content {
		name := strings.TrimRight(string(c.B_name[:]), "\x00")
		if c.B_inodo != -1 && name != "" {
			b.WriteString(fmt.Sprintf(`<TR><TD>%s</TD><TD>%d</TD></TR>`, name, c.B_inodo))
		}
	}
	b.WriteString("</TABLE>>];\n")
	return b.String()
}

// renderFileBlockNode crea el código DOT para un bloque de tipo Archivo.
func renderFileBlockNode(nodeID string, blockNumber int32, blk *Particiones.FileBlock) string {
	var b strings.Builder
	content := cleanContentForDot(string(blk.B_content[:]))

	b.WriteString(fmt.Sprintf("    %s [label=<\n", nodeID)) // Indentado para el subgráfico
	b.WriteString(`    <TABLE BORDER="1" CELLBORDER="1" CELLSPACING="0" BGCOLOR="#F5DEB3">`)
	b.WriteString(fmt.Sprintf(`<TR><TD BGCOLOR="#F39C12"><B>Bloque Archivo %d</B></TD></TR>`, blockNumber))
	b.WriteString(fmt.Sprintf(`<TR><TD ALIGN="LEFT">%s</TD></TR>`, content))
	b.WriteString("</TABLE>>];\n")
	return b.String()
}
