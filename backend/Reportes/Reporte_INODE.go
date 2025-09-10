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

func GenerarReporteInodo(pathFileLs, outputPath, id string) string {
	var output strings.Builder

	// Si pathFileLs está vacío, usar "/users.txt" como punto de partida
	if pathFileLs == "" {
		pathFileLs = "/users.txt"
	}

	// Validación básica de parámetros
	if outputPath == "" || id == "" {
		return "Error: Parámetros output o id no pueden estar vacíos"
	}

	// Normalizar el path de entrada para búsqueda y asegurar salida .jpg
	cleanPath := filepath.Clean(pathFileLs)
	if cleanPath == "." {
		return "Error: Path inválido"
	}
	outputPath = ensureJPGPath(filepath.Clean(outputPath))

	// Obtener partición montada con verificación
	mountedPartition, err := getMountedPartitionSafe(id)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	// Abrir archivo con manejo de errores
	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		return fmt.Sprintf("Error al abrir archivo: %v", err)
	}
	defer file.Close()

	// Leer superbloque con validación
	superblock, err := readSuperblockSafe(file, mountedPartition)
	if err != nil {
		return fmt.Sprintf("Error al leer superbloque: %v", err)
	}

	// Si el path es "/", generar reporte para todo el sistema de archivos
	if cleanPath == "/" {
		if err := generateFullInodeReport(file, superblock, outputPath); err != nil {
			return fmt.Sprintf("Error al generar reporte completo: %v", err)
		}
		output.WriteString(fmt.Sprintf("✅ Reporte completo de inodos generado exitosamente en: %s", outputPath))
		return output.String()
	}

	// Búsqueda segura del inodo
	inodeIndex, err := findInodeIndexSafe(cleanPath, file, superblock)
	if err != nil {
		return fmt.Sprintf("Error al buscar inodo: %v", err)
	}

	// Leer inodo con validación
	inode, err := readInodeSafe(file, superblock, inodeIndex)
	if err != nil {
		return fmt.Sprintf("Error al leer inodo: %v", err)
	}

	// Generar reporte para el inodo específico
	if err := GenerateInodeReport(inode, outputPath, inodeIndex); err != nil {
		return fmt.Sprintf("Error al generar reporte: %v", err)
	}

	output.WriteString(fmt.Sprintf("✅ Reporte de inodo generado exitosamente en: %s", outputPath))
	return output.String()
}

func getMountedPartitionSafe(id string) (Entornos.MountedPartition, error) {
	if id == "" {
		return Entornos.MountedPartition{}, fmt.Errorf("ID de partición no puede estar vacío")
	}
	for _, partitions := range Entornos.GetMountedPartitions() {
		for _, partition := range partitions {
			if partition.MountID == id {
				return partition, nil
			}
		}
	}
	return Entornos.MountedPartition{}, fmt.Errorf("no se encontró partición con ID %s", id)
}

func readSuperblockSafe(file *os.File, partition Entornos.MountedPartition) (Particiones.SuperBlock, error) {
	var sb Particiones.SuperBlock
	if file == nil {
		return sb, fmt.Errorf("archivo no puede ser nil")
	}
	if partition.MountStart < 0 {
		return sb, fmt.Errorf("posición de montaje inválida")
	}
	if err := Utils.ReadFile(file, &sb, int64(partition.MountStart)); err != nil {
		return sb, fmt.Errorf("no se pudo leer superbloque: %v", err)
	}
	// Validar valores críticos del superbloque
	if sb.S_inodes_count <= 0 || sb.S_blocks_count <= 0 {
		return sb, fmt.Errorf("valores inválidos en superbloque")
	}
	return sb, nil
}

func findInodeIndexSafe(path string, file *os.File, sb Particiones.SuperBlock) (int32, error) {
	if path == "" {
		return -1, fmt.Errorf("path no puede estar vacío")
	}
	inodeIndex, log := Usuarios.InitSearch(path, file, sb)
	if inodeIndex == -1 {
		return -1, fmt.Errorf("no se encontró el path '%s' (%s)", path, log)
	}
	// Validar rango del inodo
	if inodeIndex < 0 || inodeIndex >= sb.S_inodes_count {
		return -1, fmt.Errorf("índice de inodo %d fuera de rango (0-%d)", inodeIndex, sb.S_inodes_count-1)
	}
	return inodeIndex, nil
}

func readInodeSafe(file *os.File, sb Particiones.SuperBlock, index int32) (Particiones.Inode, error) {
	var inode Particiones.Inode
	inodeSize := int32(binary.Size(inode))

	// Validar posición del inodo
	inodePos := sb.S_inode_start + index*inodeSize
	if inodePos < sb.S_inode_start || inodePos >= sb.S_inode_start+sb.S_inodes_count*inodeSize {
		return inode, fmt.Errorf("posición de inodo %d inválida", inodePos)
	}

	// Verificar tamaño del archivo
	if fileInfo, err := file.Stat(); err == nil {
		if inodePos+inodeSize > int32(fileInfo.Size()) {
			return inode, fmt.Errorf("inodo %d está fuera del archivo", index)
		}
	}

	if err := Utils.ReadFile(file, &inode, int64(inodePos)); err != nil {
		return inode, fmt.Errorf("error al leer inodo %d: %v", index, err)
	}
	return inode, nil
}

func cleanString(b []byte) string {
	return strings.TrimRight(string(b), "\x00")
}

// ===================== GENERADORES DE .DOT + RENDER =====================

func GenerateInodeReport(inode Particiones.Inode, outputPath string, inodeNumber int32) error {
	// Crear directorio
	if err := os.MkdirAll(filepath.Dir(outputPath), os.ModePerm); err != nil {
		return fmt.Errorf("error al crear la carpeta de reportes: %v", err)
	}

	// Crear .dot
	dotFilePath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dot"
	dotFile, err := os.Create(dotFilePath)
	if err != nil {
		return fmt.Errorf("error al crear el archivo .dot de reporte: %v", err)
	}
	defer dotFile.Close()

	// Determinar el tipo de inodo para el color
	inodeType := "Archivo"
	inodeColor := "#e74c3c" // Rojo
	if inode.I_type[0] == '0' {
		inodeType = "Carpeta"
		inodeColor = "#2ecc71" // Verde
	}

	// Contenido Graphviz (usar shape=plaintext para labels HTML)
	graphContent := `digraph G {
  node [shape=plaintext];
  graph [splines=false];
`
	graphContent += fmt.Sprintf(`
  inode_table [label=<
    <TABLE BORDER="1" CELLBORDER="1" CELLSPACING="0" CELLPADDING="4">
      <TR>
        <TD COLSPAN="2" BGCOLOR="%s">
          <FONT COLOR="white"><B>Inodo %d (%s)</B></FONT>
        </TD>
      </TR>
      <TR><TD BGCOLOR="#f8f9fa"><B>i_uid</B></TD><TD BGCOLOR="#f8f9fa">%d</TD></TR>
      <TR><TD BGCOLOR="#f8f9fa"><B>i_gid</B></TD><TD BGCOLOR="#f8f9fa">%d</TD></TR>
      <TR><TD BGCOLOR="#f8f9fa"><B>i_size</B></TD><TD BGCOLOR="#f8f9fa">%d bytes</TD></TR>
      <TR><TD BGCOLOR="#f8f9fa"><B>i_atime</B></TD><TD BGCOLOR="#f8f9fa">%s</TD></TR>
      <TR><TD BGCOLOR="#f8f9fa"><B>i_ctime</B></TD><TD BGCOLOR="#f8f9fa">%s</TD></TR>
      <TR><TD BGCOLOR="#f8f9fa"><B>i_mtime</B></TD><TD BGCOLOR="#f8f9fa">%s</TD></TR>
      <TR><TD BGCOLOR="#f8f9fa"><B>i_perm</B></TD><TD BGCOLOR="#f8f9fa">%s</TD></TR>
      <TR><TD BGCOLOR="#f8f9fa"><B>i_type</B></TD><TD BGCOLOR="#f8f9fa">%s</TD></TR>
      <TR><TD COLSPAN="2" BGCOLOR="#e9ecef"><B>Bloques de datos</B></TD></TR>
`, inodeColor, inodeNumber, inodeType, inode.I_uid, inode.I_gid, inode.I_size,
		cleanString(inode.I_atime[:]), cleanString(inode.I_ctime[:]), cleanString(inode.I_mtime[:]),
		cleanString(inode.I_perm[:]), cleanString(inode.I_type[:]))

	for i, block := range inode.I_block {
		bgColor := "#ecf0f1" // Libre
		content := "-1 (vacío)"
		if block != -1 {
			bgColor = "#3498db" // Usado
			content = fmt.Sprintf("%d", block)
		}
		graphContent += fmt.Sprintf(
			`      <TR><TD BGCOLOR="#f8f9fa"><B>i_block_%d</B></TD><TD BGCOLOR="%s">%s</TD></TR>
`, i+1, bgColor, content)
	}

	graphContent += `    </TABLE>
  >];
}
`

	// Escribir .dot
	if _, err := dotFile.WriteString(graphContent); err != nil {
		return fmt.Errorf("error al escribir en el archivo .dot: %v", err)
	}

	// Render a JPG (si hay dot)
	jpgPath := strings.TrimSuffix(dotFilePath, ".dot") + ".jpg"
	if _, err := exec.LookPath("dot"); err != nil {
		// si no hay dot, solo dejamos el .dot
		return nil
	}
	if err := exec.Command("dot", "-Tjpg", dotFilePath, "-o", jpgPath).Run(); err != nil {
		return fmt.Errorf("error al convertir DOT a JPG: %v", err)
	}
	return nil
}

func generateFullInodeReport(file *os.File, sb Particiones.SuperBlock, outputPath string) error {
	// Crear directorio de salida
	if err := os.MkdirAll(filepath.Dir(outputPath), os.ModePerm); err != nil {
		return fmt.Errorf("error al crear la carpeta de reportes: %v", err)
	}

	// Crear .dot
	dotFilePath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dot"
	dotFile, err := os.Create(dotFilePath)
	if err != nil {
		return fmt.Errorf("error al crear el archivo .dot de reporte: %v", err)
	}
	defer dotFile.Close()

	// Contenido Graphviz (usar shape=plaintext para labels HTML)
	graphContent := `digraph G {
  node [shape=plaintext];
  graph [splines=false];
`
	// Recorrer todos los inodos (misma lógica que tu versión)
	for i := int32(0); i < sb.S_inodes_count; i++ {
		inode, err := readInodeSafe(file, sb, i)
		if err != nil {
			continue // Ignorar errores al leer inodos individuales
		}

		inodeType := "Archivo"
		inodeColor := "#e74c3c" // Rojo
		if inode.I_type[0] == '0' {
			inodeType = "Carpeta"
			inodeColor = "#2ecc71" // Verde
		}

		graphContent += fmt.Sprintf(`
  inode_%d [label=<
    <TABLE BORDER="1" CELLBORDER="1" CELLSPACING="0" CELLPADDING="4">
      <TR><TD COLSPAN="2" BGCOLOR="%s"><FONT COLOR="white"><B>Inodo %d (%s)</B></FONT></TD></TR>
      <TR><TD BGCOLOR="#f8f9fa"><B>i_uid</B></TD><TD BGCOLOR="#f8f9fa">%d</TD></TR>
      <TR><TD BGCOLOR="#f8f9fa"><B>i_gid</B></TD><TD BGCOLOR="#f8f9fa">%d</TD></TR>
      <TR><TD BGCOLOR="#f8f9fa"><B>i_size</B></TD><TD BGCOLOR="#f8f9fa">%d bytes</TD></TR>
    </TABLE>
  >];
`, i, inodeColor, i, inodeType, inode.I_uid, inode.I_gid, inode.I_size)
	}

	graphContent += "}\n"

	// Escribir .dot
	if _, err := dotFile.WriteString(graphContent); err != nil {
		return fmt.Errorf("error al escribir en el archivo .dot: %v", err)
	}

	// Render a JPG (si hay dot)
	jpgPath := strings.TrimSuffix(dotFilePath, ".dot") + ".jpg"
	if _, err := exec.LookPath("dot"); err != nil {
		return nil // dejamos el .dot si no hay Graphviz
	}
	if err := exec.Command("dot", "-Tjpg", dotFilePath, "-o", jpgPath).Run(); err != nil {
		return fmt.Errorf("error al convertir DOT a JPG: %v", err)
	}
	return nil
}

// ===================== UTIL =====================

func ensureJPGPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".jpg" {
		return strings.TrimSuffix(path, ext) + ".jpg"
	}
	return path
}
