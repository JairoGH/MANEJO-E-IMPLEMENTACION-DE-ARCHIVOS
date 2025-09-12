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

//
// ===================== ENTRADA PRINCIPAL =====================
//

func GenerarReporteInodo(pathFileLs, outputPath, id string) string {
	var output strings.Builder

	// Si pathFileLs está vacío, usar "/users.txt" como punto de partida
	if strings.TrimSpace(pathFileLs) == "" {
		pathFileLs = "/users.txt"
	}

	// Validaciones
	if strings.TrimSpace(outputPath) == "" || strings.TrimSpace(id) == "" {
		return "Error: Parámetros output o id no pueden estar vacíos"
	}

	// Normalizar paths
	cleanPath := filepath.Clean(pathFileLs)
	if cleanPath == "." {
		return "Error: Path inválido"
	}
	outputPath = ensureJPGPath(filepath.Clean(outputPath))

	// Partición montada
	mountedPartition, err := getMountedPartitionSafe(id)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	// Abrir archivo
	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		return fmt.Sprintf("Error al abrir archivo: %v", err)
	}
	defer file.Close()

	// Leer superbloque
	superblock, err := readSuperblockSafe(file, mountedPartition)
	if err != nil {
		return fmt.Sprintf("Error al leer superbloque: %v", err)
	}

	// Reporte de todos los inodos
	if cleanPath == "/" {
		if err := generateFullInodeReport(file, superblock, mountedPartition, outputPath); err != nil {
			return fmt.Sprintf("Error al generar reporte completo: %v", err)
		}
		output.WriteString(fmt.Sprintf("✅ Reporte completo de inodos generado exitosamente en: %s", outputPath))
		return output.String()
	}

	// Buscar inodo por path
	inodeIndex, err := findInodeIndexSafe(cleanPath, file, superblock)
	if err != nil {
		return fmt.Sprintf("Error al buscar inodo: %v", err)
	}

	// Leer inodo puntual (con heurística anti-offset)
	inode, err := readInodeSafe(file, superblock, mountedPartition, inodeIndex)
	if err != nil {
		return fmt.Sprintf("Error al leer inodo: %v", err)
	}

	// Generar reporte del inodo con expansión hacia hijos (si es carpeta)
	if err := GenerateInodeReportWithChildren(file, superblock, mountedPartition, inode, outputPath, inodeIndex); err != nil {
		return fmt.Sprintf("Error al generar reporte: %v", err)
	}

	output.WriteString(fmt.Sprintf("✅ Reporte de inodo generado exitosamente en: %s", outputPath))
	return output.String()
}

//
// ===================== HELPERS DE MONTAJE / SUPERBLOQUE =====================
//

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
	// Se asume SB al inicio de la partición (offset MountStart).
	if err := Utils.ReadFile(file, &sb, int64(partition.MountStart)); err != nil {
		return sb, fmt.Errorf("no se pudo leer superbloque: %v", err)
	}
	// Validaciones mínimas
	if sb.S_inodes_count <= 0 || sb.S_blocks_count <= 0 {
		return sb, fmt.Errorf("valores inválidos en superbloque")
	}
	return sb, nil
}

//
// ===================== BÚSQUEDA POR PATH =====================
//

func findInodeIndexSafe(path string, file *os.File, sb Particiones.SuperBlock) (int32, error) {
	if path == "" {
		return -1, fmt.Errorf("path no puede estar vacío")
	}
	inodeIndex, log := Usuarios.InitSearch(path, file, sb)
	if inodeIndex == -1 {
		return -1, fmt.Errorf("no se encontró el path '%s' (%s)", path, log)
	}
	if inodeIndex < 0 || inodeIndex >= sb.S_inodes_count {
		return -1, fmt.Errorf("índice de inodo %d fuera de rango (0-%d)", inodeIndex, sb.S_inodes_count-1)
	}
	return inodeIndex, nil
}

//
// ===================== LECTURA DE INODO (con heurística anti-offset) =====================
//

func readInodeSafe(file *os.File, sb Particiones.SuperBlock, part Entornos.MountedPartition, index int32) (Particiones.Inode, error) {
	var inode Particiones.Inode

	// Tamaño de inodo consistente con mkfs
	inodeSize := sb.S_inode_size
	if inodeSize <= 0 {
		inodeSize = int32(binary.Size(Particiones.Inode{})) // fallback
	}

	// Intento A: offsets relativos a partición (MountStart + S_inode_start)
	baseA := part.MountStart + sb.S_inode_start
	posA := baseA + index*inodeSize
	if err := readInodeAt(file, &inode, sb, posA, inodeSize); err == nil && looksSaneInode(sb, &inode) {
		return inode, nil
	}

	// Intento B: S_inode_start absoluto (sin MountStart)
	baseB := sb.S_inode_start
	posB := baseB + index*inodeSize
	if err := readInodeAt(file, &inode, sb, posB, inodeSize); err == nil && looksSaneInode(sb, &inode) {
		return inode, nil
	}

	// Si nada parece sano, devolvemos el del intento A con verificación
	if err := readInodeAt(file, &inode, sb, posA, inodeSize); err != nil {
		return inode, err
	}
	return inode, nil
}

func readInodeAt(file *os.File, inode *Particiones.Inode, sb Particiones.SuperBlock, pos int32, inodeSize int32) error {
	// Verificar límites de archivo
	if fi, err := file.Stat(); err == nil {
		if int64(pos+inodeSize) > fi.Size() {
			return fmt.Errorf("inodo fuera del archivo (offset=%d size=%d file=%d)", pos, inodeSize, fi.Size())
		}
	}
	// Leer el inodo
	return Utils.ReadFile(file, inode, int64(pos))
}

// Heurística simple para saber si lo leído "tiene sentido"
func looksSaneInode(sb Particiones.SuperBlock, in *Particiones.Inode) bool {
	if in.I_uid < 0 || in.I_uid > 1<<20 { // ~1M
		return false
	}
	if in.I_gid < 0 || in.I_gid > 1<<20 {
		return false
	}
	for _, b := range in.I_block { // -1 permitido
		if b >= sb.S_blocks_count {
			return false
		}
	}
	return true
}

//
// ===================== BITMAPS =====================
//

func isInodeUsed(file *os.File, sb Particiones.SuperBlock, part Entornos.MountedPartition, idx int32) (bool, error) {
	if idx < 0 || idx >= sb.S_inodes_count {
		return false, fmt.Errorf("índice de inodo fuera de rango")
	}
	bmStart := part.MountStart + sb.S_bm_inode_start
	byteOff := bmStart + (idx / 8)
	bitMask := byte(1 << (idx % 8))

	var b [1]byte
	if err := Utils.ReadFile(file, &b, int64(byteOff)); err != nil {
		return false, err
	}
	return (b[0] & bitMask) != 0, nil
}

func isBlockUsed(file *os.File, sb Particiones.SuperBlock, part Entornos.MountedPartition, blk int32) (bool, error) {
	if blk < 0 || blk >= sb.S_blocks_count {
		return false, fmt.Errorf("índice de bloque fuera de rango")
	}
	bmStart := part.MountStart + sb.S_bm_block_start
	byteOff := bmStart + (blk / 8)
	bitMask := byte(1 << (blk % 8))

	var b [1]byte
	if err := Utils.ReadFile(file, &b, int64(byteOff)); err != nil {
		return false, err
	}
	return (b[0] & bitMask) != 0, nil
}

//
// ===================== LECTURA DE BLOQUES DE CARPETA =====================
//

func readFolderBlock(file *os.File, sb Particiones.SuperBlock, part Entornos.MountedPartition, blkIdx int32) (Particiones.FolderBlock, error) {
	var fb Particiones.FolderBlock
	blockSize := sb.S_block_size
	if blockSize <= 0 {
		blockSize = int32(binary.Size(Particiones.FolderBlock{}))
	}
	dataStart := part.MountStart + sb.S_block_start
	pos := dataStart + blkIdx*blockSize
	if fi, err := file.Stat(); err == nil {
		if int64(pos+blockSize) > fi.Size() {
			return fb, fmt.Errorf("bloque fuera del archivo")
		}
	}
	if err := Utils.ReadFile(file, &fb, int64(pos)); err != nil {
		return fb, err
	}
	return fb, nil
}

func getFolderEntriesNames(fb Particiones.FolderBlock) [][2]interface{} {
	var out [][2]interface{}
	for _, c := range fb.B_content {
		name := strings.TrimSpace(cleanFixedBytes(c.B_name[:]))
		if c.B_inodo == -1 || name == "" || name == "." || name == ".." {
			continue
		}
		out = append(out, [2]interface{}{name, c.B_inodo})
	}
	return out
}

//
// ===================== UTIL PARA STRINGS EN DOT =====================
//

func cleanFixedBytes(b []byte) string {
	i := 0
	for ; i < len(b); i++ {
		if b[i] == 0 {
			break
		}
	}
	return string(b[:i])
}

func sanitizePrintable(s string) string {
	var out []rune
	for _, r := range s {
		if r == '\uFFFD' || (r < 0x20 && r != '\n' && r != '\t') {
			continue
		}
		out = append(out, r)
	}
	return strings.TrimSpace(string(out))
}

func gvEscape(s string) string {
	repl := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;")
	return repl.Replace(s)
}

func gvSafeTextFromBytes(b []byte) string {
	return gvEscape(sanitizePrintable(cleanFixedBytes(b)))
}

func gvSafeText(s string) string {
	return gvEscape(sanitizePrintable(s))
}

func orNA(s string) string {
	if strings.TrimSpace(s) == "" {
		return "N/A"
	}
	return s
}

//
// ===================== RENDER DE UN INODO (tabla + hijos si es carpeta) =====================
//

func GenerateInodeReportWithChildren(
	file *os.File,
	sb Particiones.SuperBlock,
	part Entornos.MountedPartition,
	inode Particiones.Inode,
	outputPath string,
	inodeNumber int32,
) error {
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

	visited := map[int32]bool{}
	var b strings.Builder

	fmt.Fprintln(&b, "digraph G {")
	fmt.Fprintln(&b, "  node [shape=plaintext, fontname=\"Helvetica\"];")
	fmt.Fprintln(&b, "  graph [splines=true];")

	// Render del nodo raíz
	renderInodeTable(&b, inodeNumber, inode)

	// Si es carpeta, expandir hijos (hasta 2 niveles por defecto)
	if inode.I_type[0] == 0 || inode.I_type[0] == '0' {
		expandDirChildren(file, sb, part, inodeNumber, inode, &b, visited, 0, 2)
	}

	fmt.Fprintln(&b, "}")

	// Escribir .dot
	if _, err := dotFile.WriteString(b.String()); err != nil {
		return fmt.Errorf("error al escribir en el archivo .dot: %v", err)
	}

	// Render a JPG (capturando stderr)
	jpgPath := strings.TrimSuffix(dotFilePath, ".dot") + ".jpg"
	if _, err := exec.LookPath("dot"); err != nil {
		return nil // si no hay graphviz, dejamos el .dot
	}
	cmd := exec.Command("dot", "-Tjpg", dotFilePath, "-o", jpgPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error al convertir DOT a JPG: %v. Detalle: %s", err, string(out))
	}
	return nil
}

// Dibuja la tabla completa de un inodo (con i_block_* incluidos)
func renderInodeTable(b *strings.Builder, idx int32, inode Particiones.Inode) {
	// Tipo de inodo
	isDir := (inode.I_type[0] == 0 || inode.I_type[0] == '0')
	inodeType := "Archivo"
	inodeColor := "#e74c3c"
	if isDir {
		inodeType = "Carpeta"
		inodeColor = "#2ecc71"
	}

	// Campos sanitizados
	atime := orNA(gvSafeTextFromBytes(inode.I_atime[:]))
	ctime := orNA(gvSafeTextFromBytes(inode.I_ctime[:]))
	mtime := orNA(gvSafeTextFromBytes(inode.I_mtime[:]))
	perm := orNA(gvSafeTextFromBytes(inode.I_perm[:]))
	itype := orNA(gvSafeTextFromBytes(inode.I_type[:]))

	fmt.Fprintf(b, `
  inode_%d [label=<
    <TABLE BORDER="1" CELLBORDER="1" CELLSPACING="0" CELLPADDING="4">
      <TR>
        <TD COLSPAN="2" BGCOLOR="%s"><FONT COLOR="white"><B>Inodo %d (%s)</B></FONT></TD>
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
`, idx, inodeColor, idx, gvSafeText(inodeType),
		inode.I_uid, inode.I_gid, inode.I_size,
		atime, ctime, mtime, perm, itype)

	for i, blk := range inode.I_block {
		bgColor := "#ecf0f1"
		content := "-1 (vacío)"
		if blk != -1 {
			bgColor = "#3498db"
			content = fmt.Sprintf("%d", blk)
		}
		fmt.Fprintf(b, `      <TR><TD BGCOLOR="#f8f9fa"><B>i_block_%d</B></TD><TD BGCOLOR="%s">%s</TD></TR>
`, i+1, bgColor, gvSafeText(content))
	}

	fmt.Fprintln(b, "    </TABLE>")
	fmt.Fprintln(b, "  >];")
}

// Expande recursivamente hijos de un directorio leyendo sus FolderBlocks
func expandDirChildren(
	file *os.File,
	sb Particiones.SuperBlock,
	part Entornos.MountedPartition,
	parentIdx int32,
	parent Particiones.Inode,
	b *strings.Builder,
	visited map[int32]bool,
	depth int,
	maxDepth int,
) {
	if depth >= maxDepth {
		return
	}

	for i, blk := range parent.I_block {
		_ = i // (solo informativo si quisieras filtrar por directos)
		if blk < 0 || blk >= sb.S_blocks_count {
			continue
		}
		// Leer bloque como carpeta
		fb, err := readFolderBlock(file, sb, part, blk)
		if err != nil {
			continue
		}
		for _, p := range getFolderEntriesNames(fb) {
			name := p[0].(string)
			childIdx := p[1].(int32)

			// Evitar bucles
			if visited[childIdx] {
				// Igual dibujamos la arista, pero no volvemos a renderizar
				fmt.Fprintf(b, `  inode_%d -> inode_%d [label="%s"];`+"\n", parentIdx, childIdx, gvSafeText(name))
				continue
			}

			// Leer inodo hijo
			child, err := readInodeSafe(file, sb, part, childIdx)
			if err != nil {
				continue
			}

			// Render nodo hijo y arista
			renderInodeTable(b, childIdx, child)
			fmt.Fprintf(b, `  inode_%d -> inode_%d [label="%s"];`+"\n", parentIdx, childIdx, gvSafeText(name))
			visited[childIdx] = true

			// Si el hijo es carpeta, expandir un nivel más
			if child.I_type[0] == 0 || child.I_type[0] == '0' {
				expandDirChildren(file, sb, part, childIdx, child, b, visited, depth+1, maxDepth)
			}
		}
	}
}

//
// ===================== REPORTE COMPLETO (tabla simple de inodos usados) =====================
//

func generateFullInodeReport(file *os.File, sb Particiones.SuperBlock, part Entornos.MountedPartition, outputPath string) error {
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

	var b strings.Builder
	fmt.Fprintln(&b, "digraph G {")
	fmt.Fprintln(&b, "  node [shape=plaintext, fontname=\"Helvetica\"];")
	fmt.Fprintln(&b, "  graph [splines=false];")

	// Solo inodos usados según bitmap
	for i := int32(0); i < sb.S_inodes_count; i++ {
		used, err := isInodeUsed(file, sb, part, i)
		if err != nil || !used {
			continue
		}
		inode, err := readInodeSafe(file, sb, part, i)
		if err != nil {
			continue
		}
		renderInodeTable(&b, i, inode)
	}

	fmt.Fprintln(&b, "}")

	// Escribir .dot
	if _, err := dotFile.WriteString(b.String()); err != nil {
		return fmt.Errorf("error al escribir en el archivo .dot: %v", err)
	}

	// Render a JPG (capturando stderr)
	jpgPath := strings.TrimSuffix(dotFilePath, ".dot") + ".jpg"
	if _, err := exec.LookPath("dot"); err != nil {
		return nil // dejamos el .dot si no hay Graphviz
	}
	cmd := exec.Command("dot", "-Tjpg", dotFilePath, "-o", jpgPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error al convertir DOT a JPG: %v. Detalle: %s", err, string(out))
	}
	return nil
}

//
// ===================== UTIL =====================
//

func ensureJPGPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".jpg" {
		return strings.TrimSuffix(path, ext) + ".jpg"
	}
	return path
}
