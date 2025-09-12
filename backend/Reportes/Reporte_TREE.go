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

// GenerarReporteArbol genera un reporte visual del árbol de inodos y bloques
func GenerarReporteArbol(diskPath, reportPath, id string) string {
	var output strings.Builder
	fmt.Printf("🔍 Depuración: Iniciando generación de reporte de árbol para ID: %s\n", id)

	// Abrir el archivo binario del disco montado
	file, err := Utils.OpenFile(diskPath)
	if err != nil {
		fmt.Printf("❌ Error al abrir el archivo: %v\n", err)
		return fmt.Sprintf("Error: No se pudo abrir el archivo en la ruta: %s", diskPath)
	}
	defer file.Close()
	fmt.Println("✅ Archivo abierto correctamente.")

	// Obtener la partición montada
	var partitionStart int64
	partitionFound := false
	for _, partitions := range Entornos.GetMountedPartitions() {
		for _, partition := range partitions {
			if partition.MountID == id {
				partitionStart = int64(partition.MountStart)
				partitionFound = true
				fmt.Printf("✅ Partición encontrada: MountStart = %d\n", partitionStart)
				break
			}
		}
		if partitionFound {
			break
		}
	}

	if !partitionFound {
		fmt.Printf("❌ Error: No se encontró la partición con ID: %s\n", id)
		return fmt.Sprintf("Error: No se encontró la partición con ID: %s", id)
	}

	// Leer el superbloque
	var superblock Particiones.SuperBlock
	if err := Utils.ReadFile(file, &superblock, partitionStart); err != nil {
		fmt.Printf("❌ Error al leer el superbloque: %v\n", err)
		return fmt.Sprintf("Error al leer superbloque: %v", err)
	}
	fmt.Printf("✅ Superbloque leído correctamente: S_filesystem_type = %d\n", superblock.S_filesystem_type)

	// Validar que es un sistema de archivos válido
	if superblock.S_filesystem_type == 0 {
		fmt.Println("❌ Error: El sistema de archivos no está formateado.")
		return "Error: El sistema de archivos no está formateado"
	}

	// Crear el archivo DOT para el árbol
	dotContent := &strings.Builder{}
	dotContent.WriteString("digraph G {\n")
	dotContent.WriteString("  rankdir=\"LR\";\n")
	dotContent.WriteString("  node [shape=record, fontname=\"Arial\", fontsize=10];\n\n")

	// Mapa para evitar procesar el mismo inodo múltiples veces
	processedInodes := make(map[int32]bool)

	// Procesar el inodo raíz (generalmente inodo 0)
	fmt.Println("🔍 Depuración: Procesando inodo raíz (Inodo 0).")
	if err := processInodeForTree(0, file, superblock, dotContent, processedInodes); err != nil {
		fmt.Printf("❌ Error al procesar inodo raíz: %v\n", err)
		return fmt.Sprintf("Error al procesar inodo raíz: %v", err)
	}

	dotContent.WriteString("}\n")

	// Depuración: Imprimir el contenido del archivo DOT generado
	fmt.Println("🔍 Depuración: Contenido del archivo DOT generado:")
	fmt.Println(dotContent.String())

	// Crear directorio si no existe
	if err := os.MkdirAll(filepath.Dir(reportPath), 0755); err != nil {
		fmt.Printf("❌ Error al crear directorio: %v\n", err)
		return fmt.Sprintf("Error al crear directorio: %v", err)
	}
	fmt.Println("✅ Directorio creado correctamente.")

	// Guardar contenido DOT
	dotFile := strings.TrimSuffix(reportPath, filepath.Ext(reportPath)) + ".dot"
	if err := os.WriteFile(dotFile, []byte(dotContent.String()), 0644); err != nil {
		fmt.Printf("❌ Error al guardar el archivo DOT: %v\n", err)
		return fmt.Sprintf("Error al guardar el archivo DOT: %v", err)
	}
	fmt.Printf("✅ Archivo DOT guardado correctamente: %s\n", dotFile)

	// Convertir el archivo DOT en imagen con Graphviz
	cmd := exec.Command("dot", "-Tjpg", dotFile, "-o", reportPath)
	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Error al convertir el archivo DOT a imagen: %v\n", err)
		output.WriteString(fmt.Sprintf("Error al convertir el archivo DOT a imagen: %v", err))
		return output.String()
	}
	fmt.Printf("✅ Imagen generada correctamente: %s\n", reportPath)
	output.WriteString(fmt.Sprintf("Reporte de árbol generado exitosamente en: %s", reportPath))

	return output.String()
}

func processInodeForTree(inodeIndex int32, file *os.File, sb Particiones.SuperBlock, dot *strings.Builder, processedInodes map[int32]bool) error {
	fmt.Printf("🔍 Depuración: Procesando inodo %d\n", inodeIndex)

	// Verificar si ya fue procesado para evitar bucles infinitos
	if processedInodes[inodeIndex] {
		fmt.Printf("⚠️ Inodo %d ya fue procesado, saltando...\n", inodeIndex)
		return nil
	}
	processedInodes[inodeIndex] = true

	// Validar que el inodo esté dentro del rango válido
	if inodeIndex < 0 || inodeIndex >= sb.S_inodes_count {
		fmt.Printf("❌ Error: Índice de inodo %d fuera de rango\n", inodeIndex)
		return fmt.Errorf("índice de inodo %d fuera de rango", inodeIndex)
	}

	// Calcular posición del inodo
	inodePos := sb.S_inode_start + inodeIndex*int32(binary.Size(Particiones.Inode{}))
	fmt.Printf("🔍 Depuración: Posición del inodo %d = %d\n", inodeIndex, inodePos)

	// Leer el inodo
	var inode Particiones.Inode
	if err := Utils.ReadFile(file, &inode, int64(inodePos)); err != nil {
		fmt.Printf("❌ Error al leer inodo %d: %v\n", inodeIndex, err)
		return fmt.Errorf("error al leer inodo %d: %v", inodeIndex, err)
	}

	// Limpiar el tipo de inodo (eliminar caracteres nulos)
	inodeType := strings.TrimRight(string(inode.I_type[:]), "\x00")
	fmt.Printf("✅ Inodo %d leído correctamente: Tipo = '%s', Tamaño = %d\n",
		inodeIndex, inodeType, inode.I_size)

	// Crear nodo para el inodo con mejor formateo
	var inodeLabel string
	if inodeType == "0" {
		inodeLabel = fmt.Sprintf("  inode%d [label=\"Inodo %d|Tipo: Directorio|Tamaño: %d bytes|Permisos: %d",
			inodeIndex, inodeIndex, inode.I_size, inode.I_perm)
	} else if inodeType == "1" {
		inodeLabel = fmt.Sprintf("  inode%d [label=\"Inodo %d|Tipo: Archivo|Tamaño: %d bytes|Permisos: %d",
			inodeIndex, inodeIndex, inode.I_size, inode.I_perm)
	} else {
		inodeLabel = fmt.Sprintf("  inode%d [label=\"Inodo %d|Tipo: %s|Tamaño: %d bytes|Permisos: %d",
			inodeIndex, inodeIndex, inodeType, inode.I_size, inode.I_perm)
	}

	// Agregar referencias a bloques
	for i, blockIndex := range inode.I_block {
		if blockIndex != -1 {
			inodeLabel += fmt.Sprintf("|<b%d> Bloque[%d]: %d", i, i, blockIndex)
		}
	}
	inodeLabel += "\"];\n\n"
	dot.WriteString(inodeLabel)

	// Procesar bloques del inodo según su tipo
	if inodeType == "0" { // Directorio
		for i, blockIndex := range inode.I_block {
			if blockIndex != -1 {
				// Crear conexión del inodo al bloque
				dot.WriteString(fmt.Sprintf("  inode%d:b%d -> block%d;\n", inodeIndex, i, blockIndex))
				// Procesar el bloque de directorio
				if err := processDirectoryBlock(blockIndex, file, sb, dot, processedInodes); err != nil {
					fmt.Printf("⚠️ Error al procesar bloque de directorio %d: %v\n", blockIndex, err)
					// Continuar con otros bloques en lugar de fallar completamente
					continue
				}
			}
		}
	} else if inodeType == "1" { // Archivo
		for i, blockIndex := range inode.I_block {
			if blockIndex != -1 {
				// Crear conexión del inodo al bloque
				dot.WriteString(fmt.Sprintf("  inode%d:b%d -> block%d;\n", inodeIndex, i, blockIndex))
				// Procesar el bloque de archivo
				if err := processFileBlock(blockIndex, file, sb, dot); err != nil {
					fmt.Printf("⚠️ Error al procesar bloque de archivo %d: %v\n", blockIndex, err)
					// Continuar con otros bloques en lugar de fallar completamente
					continue
				}
			}
		}
	}

	return nil
}

func processDirectoryBlock(blockIndex int32, file *os.File, sb Particiones.SuperBlock, dot *strings.Builder, processedInodes map[int32]bool) error {
	fmt.Printf("🔍 Depuración: Procesando bloque de directorio %d\n", blockIndex)

	// Validar que el bloque esté dentro del rango válido
	if blockIndex < 0 || blockIndex >= sb.S_blocks_count {
		return fmt.Errorf("índice de bloque %d fuera de rango", blockIndex)
	}

	// Leer el bloque de directorio
	var folderBlock Particiones.FolderBlock
	blockPos := sb.S_block_start + blockIndex*int32(binary.Size(Particiones.FolderBlock{}))
	if err := Utils.ReadFile(file, &folderBlock, int64(blockPos)); err != nil {
		return fmt.Errorf("error al leer bloque directorio %d: %v", blockIndex, err)
	}

	// Crear nodo para el bloque con mejor formateo
	dot.WriteString(fmt.Sprintf("  block%d [label=\"Bloque Directorio %d", blockIndex, blockIndex))

	// Crear puertos para cada entrada del directorio
	validEntries := 0
	for i, content := range folderBlock.B_content {
		if content.B_inodo != -1 {
			name := strings.TrimRight(string(content.B_name[:]), "\x00")
			if name != "" {
				dot.WriteString(fmt.Sprintf("|<f%d> %s (Inodo %d)", i, name, content.B_inodo))
				validEntries++
			}
		}
	}

	if validEntries == 0 {
		dot.WriteString("|Vacío")
	}

	dot.WriteString("\"];\n\n")

	// Procesar los inodos referenciados (excepto . y ..)
	for i, content := range folderBlock.B_content {
		if content.B_inodo != -1 {
			name := strings.TrimRight(string(content.B_name[:]), "\x00")
			if name != "" && name != "." && name != ".." {
				// Crear conexión del bloque al inodo
				dot.WriteString(fmt.Sprintf("  block%d:f%d -> inode%d;\n", blockIndex, i, content.B_inodo))
				// Procesar recursivamente el inodo referenciado
				if err := processInodeForTree(content.B_inodo, file, sb, dot, processedInodes); err != nil {
					fmt.Printf("⚠️ Error al procesar inodo %d referenciado desde directorio: %v\n", content.B_inodo, err)
					// Continuar con otras entradas en lugar de fallar completamente
					continue
				}
			}
		}
	}

	return nil
}

func processFileBlock(blockIndex int32, file *os.File, sb Particiones.SuperBlock, dot *strings.Builder) error {
	fmt.Printf("🔍 Depuración: Procesando bloque de archivo %d\n", blockIndex)

	// Validar que el bloque esté dentro del rango válido
	if blockIndex < 0 || blockIndex >= sb.S_blocks_count {
		return fmt.Errorf("índice de bloque %d fuera de rango", blockIndex)
	}

	// Leer el bloque de archivo
	var fileBlock Particiones.FileBlock
	blockPos := sb.S_block_start + blockIndex*int32(binary.Size(Particiones.FileBlock{}))
	if err := Utils.ReadFile(file, &fileBlock, int64(blockPos)); err != nil {
		return fmt.Errorf("error al leer bloque archivo %d: %v", blockIndex, err)
	}

	// Limpiar y filtrar el contenido del bloque
	content := cleanContent(string(fileBlock.B_content[:]))

	// Si no hay contenido visible, mostrar indicador
	if content == "" {
		content = "[Datos binarios]"
	}

	// Crear nodo para el bloque
	dot.WriteString(fmt.Sprintf("  block%d [label=\"Bloque Archivo %d|Contenido:|%s\", style=filled, fillcolor=lightblue];\n",
		blockIndex, blockIndex, content))

	return nil
}

func cleanContent(input string) string {
	var cleaned strings.Builder
	for _, r := range input {
		// Filtrar caracteres imprimibles (ASCII 32-126) y algunos caracteres especiales útiles
		if (r >= 32 && r <= 126) || r == '\n' || r == '\t' {
			cleaned.WriteRune(r)
		}
	}

	// Escapar caracteres para el formato DOT
	result := strings.ReplaceAll(cleaned.String(), "\"", "\\\"")
	result = strings.ReplaceAll(result, "\\", "\\\\")
	result = strings.ReplaceAll(result, "\n", "\\n")
	result = strings.ReplaceAll(result, "\r", "")
	result = strings.ReplaceAll(result, "\t", " ")

	// Limitar la longitud para no romper el formato del gráfico
	if len(result) > 100 {
		result = result[:100] + "..."
	}

	// Si el resultado está vacío después de la limpieza, indicarlo
	if strings.TrimSpace(result) == "" {
		result = "[Sin contenido visible]"
	}

	return result
}
