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
	var output strings.Builder // Usar strings.Builder para el log de salida

	// --- 1. Validaciones y Obtención de Partición ---
	if strings.TrimSpace(outputPath) == "" || strings.TrimSpace(id) == "" {
		output.WriteString("Error: Los parámetros -path y -id son obligatorios.\n")
		return output.String()
	}

	mountedPartition, found := Entornos.GetMountedPartitionByID(id)
	if !found {
		output.WriteString(fmt.Sprintf("Error: No se encontró la partición montada con ID %s\n", id))
		return output.String()
	}

	// --- 2. Abrir Disco y Leer Superbloque ---
	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		output.WriteString(fmt.Sprintf("Error al abrir el disco: %v\n", err))
		return output.String()
	}
	defer file.Close()

	var superblock Particiones.SuperBlock
	if err := Utils.ReadFile(file, &superblock, int64(mountedPartition.MountStart)); err != nil {
		output.WriteString(fmt.Sprintf("Error al leer el superbloque: %v\n", err))
		return output.String()
	}

	if superblock.S_magic != 0xEF53 {
		output.WriteString("Error: La partición no parece tener un sistema de archivos EXT2 válido.\n")
		return output.String()
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
		output.WriteString(fmt.Sprintf("Error al generar el grafo de inodos: %v\n", err))
		return output.String()
	}

	dotContent.WriteString("}\n")

	// --- 4. Guardar .dot y Generar Imagen ---
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		output.WriteString(fmt.Sprintf("Error al crear el directorio de salida: %v\n", err))
		return output.String()
	}

	dotFile := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dot"
	if err := os.WriteFile(dotFile, []byte(dotContent.String()), 0644); err != nil {
		output.WriteString(fmt.Sprintf("Error al guardar el archivo DOT: %v\n", err))
		return output.String()
	}

	output.WriteString(fmt.Sprintf("Reporte INODE (.dot) generado correctamente para: %s\n", dotFile))

	// Verificar 'dot' disponible
	if _, err := exec.LookPath("dot"); err != nil {
		output.WriteString("Advertencia: Graphviz no está instalado o 'dot' no está en PATH.\n")
		output.WriteString(fmt.Sprintf("Dejé el .dot en: %s\n", dotFile))
		output.WriteString(fmt.Sprintf("Para generar JPG: dot -Tjpg %s -o %s\n", dotFile, outputPath))
		return output.String()
	}

	cmd := exec.Command("dot", "-Tjpg", dotFile, "-o", outputPath)
	if err := cmd.Run(); err != nil {
		output.WriteString(fmt.Sprintf("Error al convertir DOT a JPG: %v\n", err))
		output.WriteString(fmt.Sprintf("Intenta manualmente: dot -Tjpg %s -o %s\n", dotFile, outputPath))
		return output.String()
	}

	output.WriteString(fmt.Sprintf("JPG generado localmente en: %s\n", outputPath))

	// --- 5. Subir a S3 ---
	bucketName := "proyecto2-front"
	reportS3Key := "reports/" + filepath.Base(outputPath) // outputPath es el .jpg

	publicURL, errS3 := Utils.UploadS3(bucketName, outputPath, reportS3Key)
	if errS3 != nil {
		output.WriteString(fmt.Sprintf("Error al subir el JPG a S3: %v\n", errS3))
	} else {
		output.WriteString(fmt.Sprintf("JPG subido a S3 exitosamente.\n"))
		output.WriteString(fmt.Sprintf("URL Pública: %s\n", publicURL))
	}

	return output.String()
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
