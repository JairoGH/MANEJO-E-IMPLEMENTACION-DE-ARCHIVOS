package Reportes

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Usuarios"
	"backend/Utils"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Genera reporte LS (carpeta o archivo) en JPG y guarda también el .dot
func GenerarReporteLS(pathFileLs string, outputPath string, id string) string {
	var out strings.Builder

	// Si no se especifica un path, el default es la raíz "/"
	if strings.TrimSpace(pathFileLs) == "" {
		pathFileLs = "/"
	}

	// Normalizar path lógico y quitar slash final (excepto "/")
	pathFileLs = normalizeFSPath(pathFileLs)

	if !Usuarios.IsUserLoggedIn() {
		return "Error: No hay una sesión activa. Use 'login' primero."
	}

	mp, ok := Entornos.GetMountedPartitionByID(id)
	if !ok {
		return fmt.Sprintf("Error: No se encontró la partición con ID %s montada", id)
	}

	dot, err := generateLSDotContent(*mp, pathFileLs)
	if err != nil {
		return fmt.Sprintf("Error al generar contenido LS: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Sprintf("Error al crear directorios de salida: %v", err)
	}

	dotPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dot"
	if err := os.WriteFile(dotPath, []byte(dot), 0644); err != nil {
		return fmt.Sprintf("Error al escribir archivo DOT: %v", err)
	}

	cmd := exec.Command("dot", "-Tjpg", dotPath, "-o", outputPath)
	if gvOut, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("Error al generar imagen con Graphviz: %v\nSalida: %s", err, string(gvOut))
	}

	out.WriteString("Reporte LS generado correctamente:\n")
	out.WriteString(fmt.Sprintf("  - Imagen : %s\n", outputPath))
	out.WriteString(fmt.Sprintf("  - DOT    : %s\n", dotPath))
	return out.String()
}

// generateLSDotContent es el núcleo que crea el reporte en formato DOT.
func generateLSDotContent(part Entornos.MountedPartition, logicalPath string) (string, error) {
	file, err := Utils.OpenFile(part.MountPath)
	if err != nil {
		return "", fmt.Errorf("no se pudo abrir el disco: %v", err)
	}
	defer file.Close()

	var sb Particiones.SuperBlock
	if err := Utils.ReadFile(file, &sb, int64(part.MountStart)); err != nil {
		return "", fmt.Errorf("no se pudo leer el superbloque: %v", err)
	}

	// --- LÓGICA CORREGIDA ---
	// 1. Encontrar el inodo del directorio solicitado
	inodeIdx, _ := Usuarios.InitSearch(logicalPath, file, sb)
	if inodeIdx == -1 {
		return "", fmt.Errorf("la ruta '%s' no fue encontrada", logicalPath)
	}

	var targetInode Particiones.Inode
	inodePos := sb.S_inode_start + inodeIdx*sb.S_inode_size
	if err := Utils.ReadFile(file, &targetInode, int64(inodePos)); err != nil {
		return "", fmt.Errorf("error al leer el inodo de la ruta '%s': %v", logicalPath, err)
	}

	// Verificar si la ruta es un directorio
	if targetInode.I_type[0] != '0' {
		return "", fmt.Errorf("la ruta '%s' no es un directorio", logicalPath)
	}

	// --- 2. Construir la tabla DOT ---
	var b strings.Builder
	b.WriteString("digraph G {\n")
	b.WriteString("  node [shape=plaintext];\n")
	b.WriteString(fmt.Sprintf(`  ls_table [label=<
    <TABLE BORDER="1" CELLBORDER="1" CELLSPACING="0" BGCOLOR="#F5F5F5">
      <TR><TD COLSPAN="8" BGCOLOR="#4CAF50"><B>Contenido de: %s</B></TD></TR>
      <TR>
        <TD><B>Permisos</B></TD>
        <TD><B>Dueño</B></TD>
        <TD><B>Grupo</B></TD>
        <TD><B>Tamaño</B></TD>
        <TD><B>Fecha Mod.</B></TD>
        <TD><B>Hora Mod.</B></TD>
        <TD><B>Tipo</B></TD>
        <TD><B>Nombre</B></TD>
      </TR>
`, htmlEscape(logicalPath)))

	// --- 3. Iterar solo sobre los bloques del directorio encontrado ---
	for i := 0; i < 12; i++ { // Solo bloques directos
		blockIndex := targetInode.I_block[i]
		if blockIndex == -1 {
			continue
		}

		var folderBlock Particiones.FolderBlock
		blockPos := sb.S_block_start + blockIndex*sb.S_block_size
		if err := Utils.ReadFile(file, &folderBlock, int64(blockPos)); err != nil {
			continue // Si no se puede leer el bloque, simplemente lo ignoramos
		}

		// Iterar sobre las 4 entradas de cada bloque de carpeta
		for _, entry := range folderBlock.B_content {
			entryName := strings.TrimRight(string(entry.B_name[:]), "\x00")
			childInodeIndex := entry.B_inodo

			if childInodeIndex == -1 || entryName == "" || entryName == "." || entryName == ".." {
				continue
			}

			// Leer el inodo hijo para obtener sus detalles
			var childInode Particiones.Inode
			childInodePos := sb.S_inode_start + childInodeIndex*sb.S_inode_size
			if err := Utils.ReadFile(file, &childInode, int64(childInodePos)); err != nil {
				continue
			}

			// Escribir la fila en la tabla
			writeRow(&b, &childInode, entryName)
		}
	}

	b.WriteString("    </TABLE>>];\n")
	b.WriteString("}\n")
	return b.String(), nil
}

// writeRow escribe una fila de la tabla HTML para un inodo.
func writeRow(b *strings.Builder, inode *Particiones.Inode, name string) {
	fechaMod, horaMod := parseDateTimeFlexible(inode.I_mtime[:])

	tipo := "Archivo"
	if inode.I_type[0] == '0' {
		tipo = "Carpeta"
	}

	b.WriteString(fmt.Sprintf(
		`      <TR><TD>%s</TD><TD>%d</TD><TD>%d</TD><TD>%d</TD><TD>%s</TD><TD>%s</TD><TD>%s</TD><TD>%s</TD></TR>`,
		htmlEscape(strings.TrimRight(string(inode.I_perm[:]), "\x00")),
		inode.I_uid,
		inode.I_gid,
		inode.I_size,
		htmlEscape(fechaMod),
		htmlEscape(horaMod),
		htmlEscape(tipo),
		htmlEscape(name),
	))
}

// ====================== Utilidades (sin cambios) ======================

func normalizeFSPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	p = filepath.ToSlash(p)
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimRight(p, "/")
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func htmlEscape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;", "<", "&lt;", ">", "&gt;",
		`"`, "&quot;", "'", "&#39;",
	).Replace(s)
}

func parseDateTimeFlexible(raw []byte) (string, string) {
	s := strings.TrimRight(string(raw), "\x00 ")
	if s == "" {
		return "N/A", "N/A"
	}

	// Probar varios formatos comunes, incluyendo el tuyo
	layouts := []string{
		"02/01/2006 15:04",
		"02/01/2006 15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t.Format("2006-01-02"), t.Format("15:04")
		}
	}
	return "Fecha Inválida", ""
}
