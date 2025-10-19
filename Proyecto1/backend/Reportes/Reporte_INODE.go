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

// GenerarReporteInodo crea un reporte visual que muestra la relación jerárquica
// de todos los inodos utilizados en el sistema de archivos, partiendo desde la raíz.
func GenerarReporteInodo(pathFileLs, outputPath, id string) string {
	// --- 1. Validaciones y Obtención de Partición ---
	if strings.TrimSpace(outputPath) == "" || strings.TrimSpace(id) == "" {
		return "Error: Los parámetros -path y -id son obligatorios."
	}

	mountedPartition, found := Entornos.GetMountedPartitionByID(id)
	if !found {
		return fmt.Sprintf("Error: No se encontró la partición montada con ID %s", id)
	}

	// --- 2. Abrir Disco y Leer Superbloque ---
	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		return fmt.Sprintf("Error al abrir el disco: %v", err)
	}
	defer file.Close()

	var superblock Particiones.SuperBlock
	if err := Utils.ReadFile(file, &superblock, int64(mountedPartition.MountStart)); err != nil {
		return fmt.Sprintf("Error al leer el superbloque: %v", err)
	}

	if superblock.S_magic != 0xEF53 {
		return "Error: La partición no parece tener un sistema de archivos EXT2 válido."
	}

	// --- 3. Generar Contenido del Archivo DOT ---
	var dotContent strings.Builder
	dotContent.WriteString("digraph InodeGraph {\n")
	dotContent.WriteString("  rankdir=\"LR\";\n")
	dotContent.WriteString("  node [shape=plaintext, fontname=\"Arial\"];\n")
	dotContent.WriteString("  edge [fontname=\"Arial\", fontsize=9];\n\n")

	processedInodes := make(map[int32]bool)

	// Iniciar siempre el recorrido desde el inodo raíz (0) para mostrar toda la jerarquía
	if err := generateInodeGraphRecursive(0, file, &superblock, &dotContent, processedInodes); err != nil {
		return fmt.Sprintf("Error al generar el grafo de inodos: %v", err)
	}

	dotContent.WriteString("}\n")

	// --- 4. Guardar .dot y Generar Imagen ---
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Sprintf("Error al crear el directorio de salida: %v", err)
	}

	dotFile := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dot"
	if err := os.WriteFile(dotFile, []byte(dotContent.String()), 0644); err != nil {
		return fmt.Sprintf("Error al guardar el archivo DOT: %v", err)
	}

	cmd := exec.Command("dot", "-Tjpg", dotFile, "-o", outputPath)
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("Advertencia: No se pudo generar la imagen con Graphviz: %v. El archivo .dot se guardó en %s", err, dotFile)
	}

	return fmt.Sprintf("Reporte de inodos generado exitosamente en: %s", outputPath)
}

// generateInodeGraphRecursive es la función principal que recorre y dibuja el grafo.
func generateInodeGraphRecursive(inodeIndex int32, file *os.File, sb *Particiones.SuperBlock, dot *strings.Builder, processed map[int32]bool) error {
	if processed[inodeIndex] {
		return nil // Evita recursión infinita y nodos duplicados
	}
	processed[inodeIndex] = true

	// Leer el inodo actual
	var inode Particiones.Inode
	inodePos := sb.S_inode_start + inodeIndex*sb.S_inode_size
	if err := Utils.ReadFile(file, &inode, int64(inodePos)); err != nil {
		return fmt.Errorf("no se pudo leer el inodo %d: %v", inodeIndex, err)
	}

	// Dibujar el inodo actual como una tabla
	renderInodeAsTable(inodeIndex, inode, dot)

	// Si el inodo es una carpeta, buscar y procesar a sus hijos
	if inode.I_type[0] == '0' {
		for i := 0; i < 12; i++ { // Recorrer solo bloques directos
			blockIndex := inode.I_block[i]
			if blockIndex == -1 {
				continue
			}

			var folderBlock Particiones.FolderBlock
			blockPos := sb.S_block_start + blockIndex*sb.S_block_size
			if err := Utils.ReadFile(file, &folderBlock, int64(blockPos)); err != nil {
				// Si falla la lectura de un bloque, continuamos con los demás
				fmt.Fprintf(os.Stderr, "Advertencia: no se pudo leer el bloque de carpeta %d: %v\n", blockIndex, err)
				continue
			}

			// Iterar sobre las entradas del bloque de carpeta
			for _, entry := range folderBlock.B_content {
				entryName := strings.TrimRight(string(entry.B_name[:]), "\x00")
				childInodeIndex := entry.B_inodo

				if childInodeIndex != -1 && entryName != "" && entryName != "." && entryName != ".." {
					// 1. Dibujar la flecha de conexión desde el inodo padre al hijo
					dot.WriteString(fmt.Sprintf("  inode_%d -> inode_%d [label=\"%s\"];\n", inodeIndex, childInodeIndex, entryName))

					// 2. Llamar recursivamente a la función para el inodo hijo
					generateInodeGraphRecursive(childInodeIndex, file, sb, dot, processed)
				}
			}
		}
	}
	return nil
}

// renderInodeAsTable genera el código DOT para visualizar un inodo como una tabla HTML.
func renderInodeAsTable(index int32, inode Particiones.Inode, b *strings.Builder) {
	isDir := inode.I_type[0] == '0'
	nodeType := "Archivo"
	nodeColor := "#FFDDC1" // Color para archivo
	if isDir {
		nodeType = "Directorio"
		nodeColor = "#BDE0FE" // Color para directorio
	}
	permStr := strings.TrimRight(string(inode.I_perm[:]), "\x00")

	var label strings.Builder
	label.WriteString(fmt.Sprintf(`inode_%d [label=<`, index))
	label.WriteString(fmt.Sprintf(`<TABLE BORDER="0" CELLBORDER="1" CELLSPACING="0" BGCOLOR="%s">`, nodeColor))
	label.WriteString(fmt.Sprintf(`<TR><TD COLSPAN="2"><B>Inodo %d (%s)</B></TD></TR>`, index, nodeType))
	label.WriteString(fmt.Sprintf(`<TR><TD>Permisos</TD><TD>%s</TD></TR>`, permStr))
	label.WriteString(fmt.Sprintf(`<TR><TD>Dueño (UID)</TD><TD>%d</TD></TR>`, inode.I_uid))
	label.WriteString(fmt.Sprintf(`<TR><TD>Grupo (GID)</TD><TD>%d</TD></TR>`, inode.I_gid))
	label.WriteString(fmt.Sprintf(`<TR><TD>Tamaño</TD><TD>%d bytes</TD></TR>`, inode.I_size))
	label.WriteString(fmt.Sprintf(`<TR><TD>Fecha Creación</TD><TD>%s</TD></TR>`, strings.TrimRight(string(inode.I_ctime[:]), "\x00")))

	// Mostrar los apuntadores a bloques que están en uso
	for i, blockIndex := range inode.I_block {
		if i < 12 && blockIndex != -1 {
			label.WriteString(fmt.Sprintf(`<TR><TD>Bloque[%d]</TD><TD>%d</TD></TR>`, i, blockIndex))
		}
	}

	label.WriteString(`</TABLE>>];`)
	b.WriteString(label.String() + "\n")
}
