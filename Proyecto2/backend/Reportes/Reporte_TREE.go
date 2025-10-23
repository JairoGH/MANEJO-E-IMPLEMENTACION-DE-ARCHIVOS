package Reportes

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GenerarReporteArbol genera un reporte visual del árbol de directorios y archivos.
func GenerarReporteArbol(diskPath, reportPath, id string) string {
	var output strings.Builder

	// --- 1. Obtener Partición y Validar ---
	mountedPartition, found := Entornos.GetMountedPartitionByID(id)
	if !found {
		output.WriteString(fmt.Sprintf("Error: No se encontró la partición montada con ID %s\n", id))
		return output.String()
	}

	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		output.WriteString(fmt.Sprintf("Error: No se pudo abrir el disco en la ruta: %s\n", mountedPartition.MountPath))
		return output.String()
	}
	defer file.Close()

	var superblock Particiones.SuperBlock
	if err := Utils.ReadFile(file, &superblock, int64(mountedPartition.MountStart)); err != nil {
		output.WriteString(fmt.Sprintf("Error al leer el superbloque: %v\n", err))
		return output.String()
	}

	if superblock.S_magic != 0xEF53 {
		output.WriteString("Error: La partición no parece tener un sistema de archivos EXT2 válido (magic number incorrecto).\n")
		return output.String()
	}

	// --- 2. Preparar Generador de DOT ---
	var nodesDot, edgesDot strings.Builder
	nodesDot.WriteString("digraph G {\n")
	nodesDot.WriteString("  rankdir=\"LR\";\n")
	nodesDot.WriteString("  node [shape=plaintext, fontname=\"Arial\"];\n\n")

	processed := make(map[string]bool) // Para evitar procesar elementos múltiples veces

	// --- 3. Iniciar Recorrido Recursivo desde el Inodo Raíz (0) ---
	if err := processInode(0, file, superblock, &nodesDot, &edgesDot, processed); err != nil {
		output.WriteString(fmt.Sprintf("Error al generar el árbol de archivos: %v\n", err))
		return output.String()
	}

	// --- 4. Combinar Nodos y Conexiones y Finalizar DOT ---
	nodesDot.WriteString(edgesDot.String())
	nodesDot.WriteString("}\n")

	// --- 5. Guardar Archivo .dot y Generar Imagen ---
	if err := os.MkdirAll(filepath.Dir(reportPath), 0755); err != nil {
		output.WriteString(fmt.Sprintf("Error al crear el directorio de salida: %v\n", err))
		return output.String()
	}

	dotFile := strings.TrimSuffix(reportPath, filepath.Ext(reportPath)) + ".dot"
	if err := os.WriteFile(dotFile, []byte(nodesDot.String()), 0644); err != nil {
		output.WriteString(fmt.Sprintf("Error al guardar el archivo DOT: %v\n", err))
		return output.String()
	}

	output.WriteString(fmt.Sprintf("Reporte TREE (.dot) generado correctamente para: %s\n", dotFile))

	// Verificar 'dot' disponible
	if _, err := exec.LookPath("dot"); err != nil {
		output.WriteString("Advertencia: Graphviz no está instalado o 'dot' no está en PATH.\n")
		output.WriteString(fmt.Sprintf("Dejé el .dot en: %s\n", dotFile))
		output.WriteString(fmt.Sprintf("Para generar JPG: dot -Tjpg %s -o %s\n", dotFile, reportPath))
		return output.String()
	}

	cmd := exec.Command("dot", "-Tjpg", dotFile, "-o", reportPath)
	if err := cmd.Run(); err != nil {
		output.WriteString(fmt.Sprintf("Error al convertir DOT a JPG: %v\n", err))
		output.WriteString(fmt.Sprintf("Intenta manualmente: dot -Tjpg %s -o %s\n", dotFile, reportPath))
		return output.String()
	}

	output.WriteString(fmt.Sprintf("JPG generado localmente en: %s\n", reportPath))

	// --- 6. Subir a S3 ---
	bucketName := "proyecto2-front"
	reportS3Key := "reports/" + filepath.Base(reportPath) // reportPath es el .jpg

	publicURL, errS3 := Utils.UploadS3(bucketName, reportPath, reportS3Key)
	if errS3 != nil {
		output.WriteString(fmt.Sprintf("Error al subir el JPG a S3: %v\n", errS3))
	} else {
		output.WriteString(fmt.Sprintf("JPG subido a S3 exitosamente.\n"))
		output.WriteString(fmt.Sprintf("URL Pública: %s\n", publicURL))
	}

	return output.String()
}

// processInode lee un inodo, lo dibuja en el grafo y procesa sus bloques asociados.
func processInode(inodeIndex int32, file *os.File, sb Particiones.SuperBlock, nodes *strings.Builder, edges *strings.Builder, processed map[string]bool) error {
	inodeID := fmt.Sprintf("inode_%d", inodeIndex)
	if processed[inodeID] {
		return nil
	}
	processed[inodeID] = true

	var inode Particiones.Inode
	inodePos := sb.S_inode_start + inodeIndex*int32(binary.Size(Particiones.Inode{}))
	if err := Utils.ReadFile(file, &inode, int64(inodePos)); err != nil {
		return fmt.Errorf("no se pudo leer el inodo %d: %v", inodeIndex, err)
	}

	isDir := inode.I_type[0] == '0'
	nodeType := "Archivo"
	nodeColor := "#FFDDC1" // Color para archivo
	if isDir {
		nodeType = "Directorio"
		nodeColor = "#BDE0FE" // Color para directorio
	}
	permStr := strings.TrimRight(string(inode.I_perm[:]), "\x00")

	// Dibuja el nodo del inodo como una tabla HTML
	var inodeLabel strings.Builder
	inodeLabel.WriteString(fmt.Sprintf(`%s [label=<`, inodeID))
	inodeLabel.WriteString(fmt.Sprintf(`<TABLE BORDER="0" CELLBORDER="1" CELLSPACING="0" BGCOLOR="%s">`, nodeColor))
	inodeLabel.WriteString(fmt.Sprintf(`<TR><TD COLSPAN="2"><B>Inodo %d (%s)</B></TD></TR>`, inodeIndex, nodeType))
	inodeLabel.WriteString(fmt.Sprintf(`<TR><TD>Permisos</TD><TD>%s</TD></TR>`, permStr))
	inodeLabel.WriteString(fmt.Sprintf(`<TR><TD>Tamaño</TD><TD>%d bytes</TD></TR>`, inode.I_size))

	for i, blockIndex := range inode.I_block {
		if i < 12 && blockIndex != -1 { // Solo apuntadores directos
			inodeLabel.WriteString(fmt.Sprintf(`<TR><TD>Bloque[%d]</TD><TD PORT="p%d">%d</TD></TR>`, i, i, blockIndex))
			edges.WriteString(fmt.Sprintf("  %s:p%d -> block_%d;\n", inodeID, i, blockIndex))

			var err error
			if isDir {
				err = processBlock(blockIndex, true, file, sb, nodes, edges, processed)
			} else {
				err = processBlock(blockIndex, false, file, sb, nodes, edges, processed)
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Advertencia: %v\n", err)
			}
		}
	}
	inodeLabel.WriteString(`</TABLE>>];`)
	nodes.WriteString(inodeLabel.String() + "\n")

	return nil
}

// processBlock lee un bloque, lo dibuja, y si es un directorio, procesa sus inodos hijos.
func processBlock(blockIndex int32, isDirBlock bool, file *os.File, sb Particiones.SuperBlock, nodes *strings.Builder, edges *strings.Builder, processed map[string]bool) error {
	blockID := fmt.Sprintf("block_%d", blockIndex)
	if processed[blockID] {
		return nil
	}
	processed[blockID] = true

	// Procesa como Bloque de Carpeta
	if isDirBlock {
		var folderBlock Particiones.FolderBlock
		blockPos := sb.S_block_start + blockIndex*int32(binary.Size(Particiones.FolderBlock{}))
		if err := Utils.ReadFile(file, &folderBlock, int64(blockPos)); err != nil {
			return fmt.Errorf("no se pudo leer el bloque de carpeta %d: %v", blockIndex, err)
		}

		var blockLabel strings.Builder
		blockLabel.WriteString(fmt.Sprintf(`%s [label=<`, blockID))
		blockLabel.WriteString(`<TABLE BORDER="0" CELLBORDER="1" CELLSPACING="0" BGCOLOR="#C1FFC1">`) // Verde para Bloque Carpeta
		blockLabel.WriteString(fmt.Sprintf(`<TR><TD COLSPAN="2"><B>Bloque Carpeta %d</B></TD></TR>`, blockIndex))
		blockLabel.WriteString(`<TR><TD><B>Nombre</B></TD><TD><B>Inodo Apuntado</B></TD></TR>`)

		for i, content := range folderBlock.B_content {
			name := strings.TrimRight(string(content.B_name[:]), "\x00")
			if content.B_inodo != -1 && name != "" {
				blockLabel.WriteString(fmt.Sprintf(`<TR><TD>%s</TD><TD PORT="f%d">%d</TD></TR>`, name, i, content.B_inodo))
				if name != "." && name != ".." {
					// Conexión con etiqueta (nombre del archivo/carpeta)
					edges.WriteString(fmt.Sprintf("  %s:f%d -> inode_%d [label=\"%s\"];\n", blockID, i, content.B_inodo, name))
					// Llamada recursiva para el inodo hijo
					processInode(content.B_inodo, file, sb, nodes, edges, processed)
				}
			}
		}
		blockLabel.WriteString(`</TABLE>>];`)
		nodes.WriteString(blockLabel.String() + "\n")
	} else { // Procesa como Bloque de Archivo
		var fileBlock Particiones.FileBlock
		blockPos := sb.S_block_start + blockIndex*int32(binary.Size(Particiones.FileBlock{}))
		if err := Utils.ReadFile(file, &fileBlock, int64(blockPos)); err != nil {
			return fmt.Errorf("no se pudo leer el bloque de archivo %d: %v", blockIndex, err)
		}

		content := cleanContentForDot(string(fileBlock.B_content[:]))
		nodes.WriteString(fmt.Sprintf(
			`%s [label=<
                <TABLE BORDER="0" CELLBORDER="1" CELLSPACING="0" BGCOLOR="#F5DEB3">
                    <TR><TD><B>Bloque Archivo %d</B></TD></TR>
                    <TR><TD ALIGN="LEFT">%s</TD></TR>
                </TABLE>
            >];`, blockID, blockIndex, content) + "\n",
		)
	}
	return nil
}

// cleanContentForDot prepara el texto para ser mostrado de forma segura en Graphviz.
func cleanContentForDot(input string) string {
	replacer := strings.NewReplacer(
		`&`, `&amp;`, `"`, `&quot;`, `'`, `&apos;`,
		`<`, `&lt;`, `>`, `&gt;`, "\n", `<BR/>`, "\r", "",
	)
	cleaned := strings.Trim(input, "\x00 ")
	escaped := replacer.Replace(cleaned)

	if len(escaped) > 64 {
		return escaped[:64] + "..."
	}
	if escaped == "" {
		return "[Vacío]"
	}
	return escaped
}
