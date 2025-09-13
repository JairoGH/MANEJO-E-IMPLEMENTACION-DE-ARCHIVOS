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
	"time"
)

// Genera reporte LS (carpeta o archivo) en JPG y guarda también el .dot
func GenerarReporteLS(pathFileLs string, outputPath string, id string) string {
	var out strings.Builder

	// Detectar flags embebidos en el path: /* (solo archivos), /** (recursivo)
	listFlags := parseListFlags(&pathFileLs)

	// Normalizar path lógico y quitar slash final (excepto "/")
	pathFileLs = normalizeFSPath(pathFileLs)

	// Requiere sesión
	if !Usuarios.IsUserLoggedIn() {
		return "Error: No hay una sesión activa. Use 'login' primero."
	}

	// Partición montada por ID
	mp, ok := Entornos.GetMountedPartitionByID(id)
	if !ok {
		return fmt.Sprintf("Error: No se encontró la partición con ID %s montada", id)
	}

	// Generar DOT
	dot, err := generateLSDotContent(*mp, pathFileLs, listFlags)
	if err != nil {
		return fmt.Sprintf("Error al generar contenido LS: %v", err)
	}

	// Asegurar carpeta
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Sprintf("Error al crear directorios de salida: %v", err)
	}

	// Escribir .dot
	dotPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dot"
	if err := os.WriteFile(dotPath, []byte(dot), 0o644); err != nil {
		return fmt.Sprintf("Error al escribir archivo DOT: %v", err)
	}

	// Generar imagen
	cmd := exec.Command("dot", "-Tjpg", dotPath, "-o", outputPath)
	if gvOut, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("Error al generar imagen con Graphviz: %v\nSalida: %s", err, string(gvOut))
	}

	out.WriteString("Reporte LS generado correctamente:\n")
	out.WriteString("  - Imagen : " + outputPath + "\n")
	out.WriteString("  - DOT    : " + dotPath + "\n")
	return out.String()
}

// ====================== Núcleo del reporte ======================

type lsFlags struct {
	onlyFiles bool
	recursive bool
}

func generateLSDotContent(part Entornos.MountedPartition, logicalPath string, flags lsFlags) (string, error) {
	file, err := Utils.OpenFile(part.MountPath)
	if err != nil {
		return "", fmt.Errorf("no se pudo abrir el disco: %v", err)
	}
	defer file.Close()

	// Superbloque
	var sb Particiones.SuperBlock
	if err := Utils.ReadFile(file, &sb, int64(part.MountStart)); err != nil {
		return "", fmt.Errorf("no se pudo leer el superbloque: %v", err)
	}

	// Resolver inodo objetivo (carpeta o archivo)
	var inodeIdx int32
	var log string
	if logicalPath == "/" {
		inodeIdx = 0
	} else {
		inodeIdx, log = Usuarios.InitSearch(logicalPath, file, sb)
		if inodeIdx == -1 {
			return "", fmt.Errorf("ruta no encontrada '%s': %s", logicalPath, log)
		}
	}

	// Leer inodo
	var in Particiones.Inode
	inPos := sb.S_inode_start + inodeIdx*int32(binary.Size(Particiones.Inode{}))
	if err := Utils.ReadFile(file, &in, int64(inPos)); err != nil {
		return "", fmt.Errorf("error al leer inodo en %d: %v", inPos, err)
	}

	// Armar DOT
	var b strings.Builder
	b.WriteString("digraph G {\n")
	b.WriteString("  rankdir=\"LR\";\n")
	b.WriteString("  node [shape=plaintext];\n")
	b.WriteString("  graph [fontname=\"Arial\", fontsize=10];\n")
	b.WriteString("  edge  [fontname=\"Arial\", fontsize=8];\n\n")

	b.WriteString("  ls_table [label=<\n")
	b.WriteString("    <table border='0' cellborder='1' cellspacing='0'>\n")
	b.WriteString("      <tr><td colspan='9' bgcolor='#e0e0e0'><b>Contenido de: " + htmlEscape(logicalPath) + headerSuffix(flags) + "</b></td></tr>\n")
	b.WriteString("      <tr>")
	b.WriteString("<td><b>Permisos</b></td>")
	b.WriteString("<td><b>Owner</b></td>")
	b.WriteString("<td><b>Grupo</b></td>")
	b.WriteString("<td><b>Tamaño</b></td>")
	b.WriteString("<td><b>Fecha mod</b></td>")
	b.WriteString("<td><b>Hora mod</b></td>")
	b.WriteString("<td><b>Tipo</b></td>")
	b.WriteString("<td><b>Fecha creación</b></td>")
	b.WriteString("<td><b>Nombre</b></td>")
	b.WriteString("</tr>\n")

	writeRow := func(e *Particiones.Inode, name string) {
		fechaMod, horaMod := parseDateTimeFlexible(e.I_mtime[:])
		fechaCre, _ := parseDateTimeFlexible(e.I_ctime[:]) // solo fecha de creación

		tipo := "Archivo"
		icon := "📄"
		if e.I_type[0] == '0' {
			tipo = "Carpeta"
			icon = "📁"
		}

		// Si se pidió sólo archivos, no imprimir carpetas
		if flags.onlyFiles && e.I_type[0] == '0' {
			return
		}

		b.WriteString(fmt.Sprintf(
			"      <tr><td>%s</td><td>%d</td><td>%d</td><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s %s</td></tr>\n",
			htmlEscape(trimNulls(string(e.I_perm[:]))),
			e.I_uid,
			e.I_gid,
			e.I_size,
			htmlEscape(fechaMod),
			htmlEscape(horaMod),
			htmlEscape(tipo),
			htmlEscape(fechaCre),
			icon,
			htmlEscape(name),
		))
	}

	// Walker (direct blocks) con soporte recursivo
	var walkDir func(curr *Particiones.Inode, prefix string)
	walkDir = func(curr *Particiones.Inode, prefix string) {
		for i, blk := range curr.I_block {
			if i >= 12 || blk == -1 {
				continue
			}
			blkPos := sb.S_block_start + blk*sb.S_block_size
			var fb Particiones.FolderBlock
			if err := Utils.ReadFile(file, &fb, int64(blkPos)); err != nil {
				continue
			}
			for _, e := range fb.B_content {
				if e.B_inodo == -1 {
					continue
				}
				name := trimNulls(string(e.B_name[:]))
				if name == "" || name == "." || name == ".." {
					continue
				}
				if e.B_inodo < 0 || e.B_inodo >= sb.S_inodes_count {
					continue
				}
				var child Particiones.Inode
				cPos := sb.S_inode_start + e.B_inodo*int32(binary.Size(Particiones.Inode{}))
				if err := Utils.ReadFile(file, &child, int64(cPos)); err != nil {
					continue
				}
				rel := name
				if prefix != "" {
					rel = prefix + name
				}
				writeRow(&child, rel)
				// Si es carpeta y se pidió recursivo, seguir bajando
				if flags.recursive && child.I_type[0] == '0' {
					walkDir(&child, rel+"/")
				}
			}
		}
	}

	// Directorio o archivo
	if in.I_type[0] == '0' {
		walkDir(&in, "")
	} else {
		// Archivo: una sola fila (mantenemos compatibilidad)
		writeRow(&in, baseNameFS(logicalPath))
	}

	b.WriteString("    </table>\n")
	b.WriteString("  >];\n")
	b.WriteString("}\n")
	return b.String(), nil
}

// ====================== Utilidades ======================

func parseListFlags(p *string) lsFlags {
	path := *p
	flags := lsFlags{}
	// /**  -> recursivo (y por consistencia, se listarán archivos; las carpetas se recorren)
	if strings.HasSuffix(path, "/**") {
		flags.recursive = true
		path = strings.TrimSuffix(path, "/**")
	}
	// /* -> solo archivos de ese directorio
	if strings.HasSuffix(path, "/*") {
		flags.onlyFiles = true
		path = strings.TrimSuffix(path, "/*")
	}
	*p = path
	return flags
}

func headerSuffix(f lsFlags) string {
	if f.recursive {
		return " (recursivo)"
	}
	if f.onlyFiles {
		return " (solo archivos)"
	}
	return ""
}

func normalizeFSPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	p = filepath.ToSlash(p)
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	// quitar slash final excepto raíz
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimRight(p, "/")
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func baseNameFS(p string) string {
	if p == "/" {
		return "/"
	}
	if i := strings.LastIndex(p, "/"); i >= 0 && i+1 < len(p) {
		return p[i+1:]
	}
	return p
}

func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&#39;",
	)
	return r.Replace(s)
}

func trimNulls(s string) string { return strings.TrimRight(strings.TrimSpace(s), "\x00") }

// -------- Fechas --------
func parseDateTimeFlexible(raw []byte) (string, string) {
	s := trimNulls(string(raw))
	if s == "" {
		return "N/A", "N/A"
	}
	s = strings.Join(strings.Fields(s), " ")
	if strings.HasSuffix(s, ":") {
		s = strings.TrimSuffix(s, ":")
	}

	parse := func(layout string) (time.Time, bool) {
		t, err := time.ParseInLocation(layout, s, time.Local)
		return t, err == nil
	}

	// dd/mm/yyyy HH:MM[:SS]
	if t, ok := parse("02/01/2006 15:04:05"); ok {
		return t.Format("02/01/2006"), t.Format("15:04")
	}
	if t, ok := parse("02/01/2006 15:04"); ok {
		return t.Format("02/01/2006"), t.Format("15:04")
	}
	// dd/mm/yyyy
	if t, ok := parse("02/01/2006"); ok {
		return t.Format("02/01/2006"), "00:00"
	}
	// yyyy-mm-dd HH:MM[:SS]
	if t, ok := parse("2006-01-02 15:04:05"); ok {
		return t.Format("02/01/2006"), t.Format("15:04")
	}
	if t, ok := parse("2006-01-02 15:04"); ok {
		return t.Format("02/01/2006"), t.Format("15:04")
	}
	// yyyy-mm-dd
	if t, ok := parse("2006-01-02"); ok {
		return t.Format("02/01/2006"), "00:00"
	}
	return "N/A", "N/A"
}
