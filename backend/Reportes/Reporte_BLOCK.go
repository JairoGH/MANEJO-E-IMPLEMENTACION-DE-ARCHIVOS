package Reportes

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Usuarios"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ==============================
// REPORTE: Bloques encadenados (un solo grafo)
// ==============================
func GenerarReporteBloques(pathFileLs string, outputPath string, id string) string {
	var sb strings.Builder

	// Path por defecto
	if strings.TrimSpace(pathFileLs) == "" {
		pathFileLs = "/users.txt"
	}
	if strings.TrimSpace(outputPath) == "" {
		return "Error: Debe indicar -path de salida (imagen .jpg/.png)"
	}

	// Obtener partición montada
	mp, found := Entornos.GetMountedPartitionByID(id)
	if !found {
		return fmt.Sprintf("Error: No se encontró la partición montada con ID %s", id)
	}

	// Abrir disco
	f, err := Utils.OpenFile(mp.MountPath)
	if err != nil {
		return fmt.Sprintf("Error abriendo disco: %v", err)
	}
	defer f.Close()

	// Leer MBR
	var mbr Particiones.MBR
	if err := Utils.ReadFile(f, &mbr, 0); err != nil {
		return "Error leyendo MBR"
	}

	// Buscar part. con ese ID y montada
	idx := -1
	for i := 0; i < 4; i++ {
		if mbr.MBR_Partition[i].Part_Size != 0 &&
			strings.Contains(string(mbr.MBR_Partition[i].Part_ID[:]), id) {
			if mbr.MBR_Partition[i].Part_Status[0] == '1' {
				idx = i
			} else {
				return "Error: la partición no está montada"
			}
			break
		}
	}
	if idx == -1 {
		return "Error: no se encontró la partición"
	}

	// Leer Superbloque
	var sbk Particiones.SuperBlock
	if err := Utils.ReadFile(f, &sbk, int64(mbr.MBR_Partition[idx].Part_Start)); err != nil {
		return "Error leyendo Superbloque"
	}

	// Resolver inodo del path
	inodeNum, _ := Usuarios.InitSearch(pathFileLs, f, sbk)
	if inodeNum == -1 {
		return fmt.Sprintf("Error: no se encontró el inodo para %s", pathFileLs)
	}

	// Leer inodo
	var ino Particiones.Inode
	inodeStart := sbk.S_inode_start + inodeNum*int32(binary.Size(Particiones.Inode{}))
	if err := Utils.ReadFile(f, &ino, int64(inodeStart)); err != nil {
		return fmt.Sprintf("Error leyendo inodo %d: %v", inodeNum, err)
	}

	// Asegurar carpeta de salida y preparar nombres
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Sprintf("Error creando carpeta de salida: %v", err)
	}
	dotPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dot"

	// ========== Construir DOT único ==========
	var dot strings.Builder
	dot.WriteString("digraph BlocksChain {\n")
	dot.WriteString("  rankdir=LR;\n")
	dot.WriteString("  node [shape=plaintext];\n")
	dot.WriteString(fmt.Sprintf("  label=\"Bloques de %s (inode %d)\";\n", pathFileLs, inodeNum))

	prevNode := ""
	wroteAtLeastOne := false

	for i := 0; i < len(ino.I_block); i++ {
		if ino.I_block[i] == -1 {
			continue
		}
		blkNum := ino.I_block[i]

		// Leer bloque de carpeta (ajústalo si tu inodo apunta a otros tipos)
		var fb Particiones.FolderBlock
		blkStart := sbk.S_block_start + blkNum*int32(binary.Size(Particiones.FolderBlock{}))
		if err := Utils.ReadFile(f, &fb, int64(blkStart)); err != nil {
			// si no se puede leer como carpeta, sigue al siguiente
			continue
		}

		nodeID := fmt.Sprintf("bl%d", blkNum)
		dot.WriteString(renderFolderBlockNode(nodeID, blkNum, &fb))

		if prevNode != "" {
			// Conectar en cadena
			dot.WriteString(fmt.Sprintf("  %s -> %s;\n", prevNode, nodeID))
		}
		prevNode = nodeID
		wroteAtLeastOne = true
	}

	if !wroteAtLeastOne {
		return "No hubo bloques válidos para graficar en este inodo."
	}

	dot.WriteString("}\n")

	// Guardar DOT
	if err := os.WriteFile(dotPath, []byte(dot.String()), 0o644); err != nil {
		return fmt.Sprintf("Error escribiendo DOT: %v", err)
	}

	// Generar imagen con Graphviz
	cmd := exec.Command("dot", "-Tjpg", dotPath, "-o", outputPath)
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("Error generando imagen: %v", err)
	}

	sb.WriteString("===== REPORTE DE BLOQUES (encadenado) =====\n")
	sb.WriteString(fmt.Sprintf("Entrada: %s\n", pathFileLs))
	sb.WriteString(fmt.Sprintf("Salida : %s\n", outputPath))
	sb.WriteString("===========================================\n")
	return sb.String()
}

// Renderiza un nodo/tabla para un FolderBlock en Graphviz
func renderFolderBlockNode(nodeID string, blockNumber int32, blk *Particiones.FolderBlock) string {
	tableColor := "#f39c12"
	headerColor := "#2ecc71"
	rowEven := "#ecf0f1"
	rowOdd := "#bdc3c7"

	var b strings.Builder
	b.WriteString(fmt.Sprintf("  %s [label=<\n", nodeID))
	b.WriteString(fmt.Sprintf("<TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" BGCOLOR=\"%s\">\n", tableColor))
	b.WriteString(fmt.Sprintf("<TR><TD COLSPAN=\"2\" BGCOLOR=\"%s\"><B>Bloque Carpeta %d</B></TD></TR>\n", headerColor, blockNumber))
	b.WriteString("<TR><TD><B>b_name</B></TD><TD><B>b_inodo</B></TD></TR>\n")

	for i, c := range blk.B_content {
		rowColor := rowEven
		if i%2 == 1 {
			rowColor = rowOdd
		}
		name := cleanString(c.B_name[:])
		if c.B_inodo != -1 && name != "" {
			b.WriteString(fmt.Sprintf("<TR BGCOLOR=\"%s\"><TD>%s</TD><TD>%d</TD></TR>\n", rowColor, name, c.B_inodo))
		} else {
			b.WriteString(fmt.Sprintf("<TR BGCOLOR=\"%s\"><TD>-</TD><TD>-</TD></TR>\n", rowColor))
		}
	}
	b.WriteString("</TABLE>\n>];\n")
	return b.String()
}

// Limpia strings con bytes nulos
func cleanString(b []byte) string {
	return strings.Trim(string(b), "\x00 \t\r\n")
}
