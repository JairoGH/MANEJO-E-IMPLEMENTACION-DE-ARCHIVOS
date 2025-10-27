package main

import (
	"backend/Analizador"
	"backend/Entornos"
	"backend/Particiones"
	"backend/Reportes"
	"backend/Usuarios"
	"backend/Utils"
	"bytes"
	"encoding/binary"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

/*************** Tipos API ***************/
type CommandRequest struct {
	Command string `json:"command"`
}
type CommandResponse struct {
	Output string `json:"output"`
}

type DiskInfo struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	SizeBytes  int64    `json:"sizeBytes"`
	SizePretty string   `json:"sizePretty"`
	Fit        string   `json:"fit"`
	Mounted    []string `json:"mounted"`
}
type PartitionInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"` // P/E/L
	Status     string `json:"status"`
	Fit        string `json:"fit"`
	Start      int64  `json:"start"`
	SizeBytes  int64  `json:"sizeBytes"`
	SizePretty string `json:"sizePretty"`
}
type DirEntry struct {
	Name   string `json:"name"`
	Type   string `json:"type"` // "dir" | "file"
	Size   int64  `json:"size"` // bytes (solo archivo)
	Perm   string `json:"perm"` // "775 (rwxrwxr-x)"
	UID    int32  `json:"uid"`
	GID    int32  `json:"gid"`
	Inode  int32  `json:"inode"`
	Target string `json:"target,omitempty"`
}

/*************** Tipos de reportes ***************/
type genReq struct {
	ID   string `json:"id"`
	Name string `json:"name"`           // blocks|ls|sb|inodes|tree|mbr|bm_blocks|bm_inodes|disk|file
	Ruta string `json:"ruta,omitempty"` // para algunos reportes (ls, blocks, file, inodes)
}
type genResp struct {
	Ok          bool   `json:"ok"`
	PublicURL   string `json:"publicUrl,omitempty"`
	LocalPath   string `json:"localPath,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Message     string `json:"message,omitempty"`
}

/*************** Utils ***************/
func humanizeBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
func trimCString(b []byte) string {
	s := string(bytes.Trim(b, "\x00"))
	return strings.TrimSpace(s)
}
func mbrSize(mbr *Particiones.MBR) int64 { return int64(mbr.MBR_Tamano) }
func mbrFit(mbr *Particiones.MBR) string {
	fit := strings.TrimSpace(string(mbr.MBR_DiskFit[:]))
	if fit == "" {
		fit = "-"
	}
	return fit
}
func buildMountedByDisk() map[string][]string {
	mp := Entornos.GetMountedPartitions()
	out := map[string][]string{}
	for _, lst := range mp {
		for _, p := range lst {
			out[p.MountPath] = append(out[p.MountPath], p.MountID)
		}
	}
	return out
}

// Detecta carpeta de discos (.mia)
func detectDisksDir(queryOverride string) (string, []string) {
	tested := []string{}
	if s := strings.TrimSpace(queryOverride); s != "" {
		tested = append(tested, s)
		if st, err := os.Stat(s); err == nil && st.IsDir() {
			return s, tested
		}
	}
	if base := strings.TrimSpace(os.Getenv("GODISK_DIR")); base != "" {
		tested = append(tested, base)
		if st, err := os.Stat(base); err == nil && st.IsDir() {
			return base, tested
		}
	}
	for _, lst := range Entornos.GetMountedPartitions() {
		for _, p := range lst {
			dir := filepath.Dir(p.MountPath)
			tested = append(tested, dir)
			if st, err := os.Stat(dir); err == nil && st.IsDir() {
				return dir, tested
			}
		}
	}
	candidates := []string{
		"/home/jairogo/Calificacion_MIA/Discos",
		"/home/jairogo/escritorio/discos",
		"./Discos",
		"/home/ubuntu/Calificacion_MIA/Discos",
		"/home/ubuntu/escritorio/discos",
		"/discos",
	}
	for _, c := range candidates {
		tested = append(tested, c)
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, tested
		}
	}
	return "", tested
}

// Particiones helpers
func partitionsFromMBR(mbr *Particiones.MBR) []PartitionInfo {
	res := []PartitionInfo{}
	for i := 0; i < 4; i++ {
		p := mbr.MBR_Partition[i]
		if p.Part_Size == 0 {
			continue
		}
		name := trimCString(p.Part_Name[:])
		ptype := string(p.Part_Type[:])
		fit := strings.TrimSpace(string(p.Part_Fit[:]))
		status := "0"
		if len(p.Part_Status) > 0 {
			status = string(p.Part_Status[:])
		}
		if ptype == "" {
			ptype = "p"
		}
		info := PartitionInfo{
			Name:       name,
			Type:       strings.ToUpper(ptype[:1]),
			Status:     status,
			Fit:        fit,
			Start:      int64(p.Part_Start),
			SizeBytes:  int64(p.Part_Size),
			SizePretty: humanizeBytes(int64(p.Part_Size)),
		}
		res = append(res, info)
	}
	return res
}
func openDisk(path string) (*os.File, error) { return Utils.OpenFile(path) }
func readMBR(f *os.File) (*Particiones.MBR, error) {
	var m Particiones.MBR
	if err := Utils.ReadFile(f, &m, 0); err != nil {
		return nil, err
	}
	return &m, nil
}
func findPartitionByName(mbr *Particiones.MBR, name string) *PartitionInfo {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, p := range partitionsFromMBR(mbr) {
		if strings.ToLower(p.Name) == name {
			return &p
		}
	}
	return nil
}
func readSuperblock(f *os.File, start int64) (Particiones.SuperBlock, error) {
	var sb Particiones.SuperBlock
	err := Utils.ReadFile(f, &sb, start)
	return sb, err
}
func rwxFromDigit(d byte) string {
	v := d - '0'
	r, w, x := '-', '-', '-'
	if v&4 != 0 {
		r = 'r'
	}
	if v&2 != 0 {
		w = 'w'
	}
	if v&1 != 0 {
		x = 'x'
	}
	return string([]rune{r, w, x})
}
func permString(in Particiones.Inode) string {
	digits := strings.TrimSpace(string(in.I_perm[:]))
	if digits == "" || !utf8.ValidString(digits) {
		digits = "000"
	}
	for len(digits) < 3 {
		digits += "0"
	}
	rwx := rwxFromDigit(digits[0]) + rwxFromDigit(digits[1]) + rwxFromDigit(digits[2])
	return fmt.Sprintf("%s (%s)", digits[:3], rwx)
}
func getInodeAt(f *os.File, sb Particiones.SuperBlock, idx int32) (Particiones.Inode, error) {
	var in Particiones.Inode
	inodeSize := sb.S_inode_size
	if inodeSize <= 0 {
		inodeSize = int32(binary.Size(Particiones.Inode{}))
	}
	pos := sb.S_inode_start + idx*inodeSize
	if err := Utils.ReadFile(f, &in, int64(pos)); err != nil {
		return Particiones.Inode{}, err
	}
	return in, nil
}
func listDirectoryEntries(f *os.File, sb Particiones.SuperBlock, dirInode Particiones.Inode) ([]DirEntry, error) {
	out := []DirEntry{}
	blockSize := sb.S_block_size
	if blockSize <= 0 {
		blockSize = int32(binary.Size(Particiones.FolderBlock{}))
	}
	for i := 0; i < 12; i++ {
		blockIndex := dirInode.I_block[i]
		if blockIndex < 0 {
			continue
		}
		var fb Particiones.FolderBlock
		pos := sb.S_block_start + blockIndex*blockSize
		if err := Utils.ReadFile(f, &fb, int64(pos)); err != nil {
			fmt.Printf("Warning: read folder block %d: %v\n", blockIndex, err)
			continue
		}
		for j := 0; j < 4; j++ {
			entry := fb.B_content[j]
			if entry.B_inodo < 0 {
				continue
			}
			name := trimCString(entry.B_name[:])
			if name == "" || name == "." || name == ".." {
				continue
			}
			in, err := getInodeAt(f, sb, entry.B_inodo)
			if err != nil {
				fmt.Printf("Warning: read inode %d for '%s': %v\n", entry.B_inodo, name, err)
				continue
			}
			etype := "file"
			if in.I_type[0] == '0' {
				etype = "dir"
			}
			out = append(out, DirEntry{
				Name:  name,
				Type:  etype,
				Size:  int64(in.I_size),
				Perm:  permString(in),
				UID:   in.I_uid,
				GID:   in.I_gid,
				Inode: entry.B_inodo,
			})
		}
	}
	return out, nil
}

/*************** Report helpers (sandbox) ***************/
var reportsBase = func() string {
	if v := strings.TrimSpace(os.Getenv("GODISK_REPORTS_DIR")); v != "" {
		return v
	}
	absPath, err := filepath.Abs("./GeneratedReports")
	if err != nil {
		fmt.Println("Warning: using relative './GeneratedReports'")
		return "./GeneratedReports"
	}
	return absPath
}()

func allowedRoots() []string { return []string{reportsBase} }
func inSandbox(p string) bool {
	pAbs, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	for _, root := range allowedRoots() {
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		if strings.HasPrefix(pAbs, rootAbs) {
			rel, err := filepath.Rel(rootAbs, pAbs)
			if err == nil && !strings.HasPrefix(rel, "..") {
				return true
			}
		}
	}
	return false
}

var reS3 = regexp.MustCompile(`(?i)URL\s+Pública:\s*(https?://\S+)`)

func extractS3URL(log string) string {
	if m := reS3.FindStringSubmatch(log); len(m) >= 2 {
		return m[1]
	}
	return ""
}
func contentTypeByExt(p string) string {
	ext := strings.ToLower(filepath.Ext(p))
	if ct := mime.TypeByExtension(ext); ct != "" {
		if strings.HasPrefix(ct, "text/") && !strings.Contains(ct, "charset") {
			return ct + "; charset=utf-8"
		}
		return ct
	}
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".pdf":
		return "application/pdf"
	case ".dot":
		return "text/vnd.graphviz"
	default:
		return "application/octet-stream"
	}
}

/*************** Helpers de endpoint ***************/
func listPartitionsForDisk(diskPath string) ([]PartitionInfo, error) {
	f, err := openDisk(diskPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", diskPath, err)
	}
	defer f.Close()
	mbr, err := readMBR(f)
	if err != nil {
		return nil, fmt.Errorf("read MBR: %w", err)
	}
	return partitionsFromMBR(mbr), nil
}

/*************** main ***************/
func init() {
	_ = os.MkdirAll(reportsBase, 0o755)
	fmt.Printf("📂 Base directory for local reports: %s\n", reportsBase)
}

func main() {
	app := fiber.New()
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, OPTIONS",
	}))

	/* ---------- health ---------- */
	app.Get("/health", func(c *fiber.Ctx) error { return c.SendString("OK") })

	/* ---------- ejecutar comandos (consola) ---------- */
	app.Post("/execute", func(c *fiber.Ctx) error {
		var req CommandRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(CommandResponse{Output: "Error: Petición inválida"})
		}
		script := strings.ReplaceAll(req.Command, "\r\n", "\n")
		lines := strings.Split(script, "\n")

		var out bytes.Buffer
		for i, raw := range lines {
			line := strings.TrimSpace(raw)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "#") {
				out.WriteString(line + "\n")
				continue
			}
			cmd, params := Analizador.GetInput(line)
			if cmd == "" {
				out.WriteString(fmt.Sprintf("Error line %d: línea no reconocida: '%s'\n", i+1, raw))
				continue
			}
			result := Analizador.AnalyzerCommand(cmd, params)
			if !strings.HasSuffix(result, "\n") {
				result += "\n"
			}
			out.WriteString(result)
		}
		output := out.String()
		if strings.TrimSpace(output) == "" {
			output = "No se ejecutó ningún comando válido.\n"
		}
		return c.JSON(CommandResponse{Output: output})
	})

	/* ---------- discos / particiones ---------- */
	app.Get("/disks", func(c *fiber.Ctx) error {
		override := c.Query("dir")
		base, tested := detectDisksDir(override)
		fmt.Printf("[/disks] Probed: %v | Using: %s\n", tested, base)
		c.Set("X-Disks-Dir", base)
		if base == "" {
			c.Set("X-Disks-Error", "No valid disk directory found.")
			return c.JSON([]DiskInfo{})
		}
		entries, err := os.ReadDir(base)
		if err != nil {
			fmt.Printf("read dir %s: %v\n", base, err)
			c.Set("X-Disks-Error", err.Error())
			return c.JSON([]DiskInfo{})
		}

		mountedByDisk := buildMountedByDisk()
		var result []DiskInfo
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".mia") {
				continue
			}
			name := e.Name()
			fullPath := filepath.Join(base, name)
			f, err := openDisk(fullPath)
			if err != nil {
				fmt.Printf("skip %s: %v\n", name, err)
				continue
			}
			mbr, err := readMBR(f)
			_ = f.Close()
			if err != nil {
				fmt.Printf("skip %s: bad MBR: %v\n", name, err)
				continue
			}
			size := mbrSize(mbr)
			if size <= 0 {
				continue
			}
			result = append(result, DiskInfo{
				Name:       name,
				Path:       fullPath,
				SizeBytes:  size,
				SizePretty: humanizeBytes(size),
				Fit:        mbrFit(mbr),
				Mounted:    mountedByDisk[fullPath],
			})
		}
		return c.JSON(result)
	})

	app.Get("/disk/partitions", func(c *fiber.Ctx) error {
		diskPath := c.Query("diskPath")
		if diskPath == "" {
			diskPath = c.Query("path")
		}
		if diskPath == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "'diskPath' query parameter is required"})
		}
		parts, err := listPartitionsForDisk(filepath.Clean(diskPath))
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(parts)
	})

	/* ---------- viewer (solo lectura) ---------- */
	app.Get("/viewer/list", func(c *fiber.Ctx) error {
		diskPath := c.Query("diskPath")
		partName := c.Query("partName")
		path := c.Query("path", "/")
		if diskPath == "" || partName == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "'diskPath' and 'partName' are required"})
		}
		f, err := openDisk(diskPath)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer f.Close()
		mbr, err := readMBR(f)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to read MBR"})
		}
		pInfo := findPartitionByName(mbr, partName)
		if pInfo == nil {
			return c.Status(404).JSON(fiber.Map{"error": fmt.Sprintf("Partition '%s' not found", partName)})
		}
		sb, err := readSuperblock(f, pInfo.Start)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("Failed to read Superblock: %v", err)})
		}
		inodeIdx, _ := Usuarios.InitSearch(path, f, sb)
		if inodeIdx < 0 {
			return c.Status(404).JSON(fiber.Map{"error": fmt.Sprintf("Path '%s' not found", path)})
		}
		in, err := getInodeAt(f, sb, inodeIdx)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if in.I_type[0] == '1' {
			return c.JSON([]DirEntry{{
				Name: filepath.Base(path), Type: "file", Size: int64(in.I_size),
				Perm: permString(in), UID: in.I_uid, GID: in.I_gid, Inode: inodeIdx,
			}})
		}
		if in.I_type[0] == '0' {
			entries, err := listDirectoryEntries(f, sb, in)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(entries)
		}
		return c.Status(500).JSON(fiber.Map{"error": "Unknown inode type"})
	})

	app.Get("/viewer/file", func(c *fiber.Ctx) error {
		diskPath := c.Query("diskPath")
		partName := c.Query("partName")
		path := c.Query("path")
		if diskPath == "" || partName == "" || path == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "'diskPath', 'partName', and 'path' are required"})
		}
		f, err := openDisk(diskPath)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer f.Close()
		mbr, err := readMBR(f)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to read MBR"})
		}
		pInfo := findPartitionByName(mbr, partName)
		if pInfo == nil {
			return c.Status(404).JSON(fiber.Map{"error": fmt.Sprintf("Partition '%s' not found", partName)})
		}
		sb, err := readSuperblock(f, pInfo.Start)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("Failed to read Superblock: %v", err)})
		}
		inodeIdx, _ := Usuarios.InitSearch(path, f, sb)
		if inodeIdx < 0 {
			return c.Status(404).JSON(fiber.Map{"error": fmt.Sprintf("File '%s' not found", path)})
		}
		in, err := getInodeAt(f, sb, inodeIdx)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if in.I_type[0] != '1' {
			return c.Status(400).JSON(fiber.Map{"error": "Path is not a file"})
		}
		data, _ := Usuarios.GetInodeFileData(in, f, sb)
		return c.JSON(fiber.Map{
			"name": filepath.Base(path), "size": in.I_size, "perm": permString(in),
			"uid": in.I_uid, "gid": in.I_gid, "inode": inodeIdx, "content": data,
		})
	})

	/*************** REPORTES (✅ incluye LS) ***************/
	app.Post("/reports/generate", func(c *fiber.Ctx) error {
		var req genReq
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(genResp{Ok: false, Message: fmt.Sprintf("Invalid JSON: %v", err)})
		}
		req.Name = strings.ToLower(strings.TrimSpace(req.Name))
		req.ID = strings.TrimSpace(req.ID)
		req.Ruta = strings.TrimSpace(req.Ruta)

		if req.Name == "" {
			return c.Status(fiber.StatusBadRequest).JSON(genResp{Ok: false, Message: "Report 'name' is required"})
		}
		if err := os.MkdirAll(reportsBase, 0o755); err != nil {
			return c.Status(500).JSON(genResp{Ok: false, Message: "Failed to prepare reports directory"})
		}

		stamp := time.Now().Format("20060102_150405")
		var outBaseName, expectedExt string
		var outputLog, finalOutPath string

		switch req.Name {
		/* ---------- LS (este es el solicitado) ---------- */
		case "ls":
			// Requiere: id (partición montada). Ruta lógica opcional (default "/")
			if req.ID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(genResp{Ok: false, Message: "'id' is required for 'ls' report"})
			}
			outBaseName = fmt.Sprintf("ls_%s_%s", req.ID, stamp)
			expectedExt = ".jpg" // Generar imagen (y .dot como subproducto)
			finalOutPath = filepath.Join(reportsBase, outBaseName+expectedExt)
			// (pathFileLs, outputPath, id)
			outputLog = Reportes.GenerarReporteLS(req.Ruta, finalOutPath, req.ID)

		/* ---------- Extras para no romper tu UI ---------- */
		case "mbr":
			if req.ID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(genResp{Ok: false, Message: "'id' is required for 'mbr' report"})
			}
			mp, found := Entornos.GetMountedPartitionByID(req.ID)
			if !found {
				return c.Status(404).JSON(genResp{Ok: false, Message: "Mounted partition not found"})
			}
			outBaseName = fmt.Sprintf("mbr_%s_%s", req.ID, stamp)
			expectedExt = ".jpg"
			finalOutPath = filepath.Join(reportsBase, outBaseName+expectedExt)
			outputLog = Reportes.GenerarReporteMBR(mp.MountPath, finalOutPath)

		case "sb":
			if req.ID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(genResp{Ok: false, Message: "'id' is required for 'sb' report"})
			}
			mp, found := Entornos.GetMountedPartitionByID(req.ID)
			if !found {
				return c.Status(404).JSON(genResp{Ok: false, Message: "Mounted partition not found"})
			}
			outBaseName = fmt.Sprintf("sb_%s_%s", req.ID, stamp)
			expectedExt = ".jpg"
			finalOutPath = filepath.Join(reportsBase, outBaseName+expectedExt)
			outputLog = Reportes.GenerarReporteSB(mp.MountPath, finalOutPath, req.ID)

		case "tree":
			if req.ID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(genResp{Ok: false, Message: "'id' is required for 'tree' report"})
			}
			mp, found := Entornos.GetMountedPartitionByID(req.ID)
			if !found {
				return c.Status(404).JSON(genResp{Ok: false, Message: "Mounted partition not found"})
			}
			outBaseName = fmt.Sprintf("tree_%s_%s", req.ID, stamp)
			expectedExt = ".jpg"
			finalOutPath = filepath.Join(reportsBase, outBaseName+expectedExt)
			outputLog = Reportes.GenerarReporteArbol(mp.MountPath, finalOutPath, req.ID)

		case "inodes":
			if req.ID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(genResp{Ok: false, Message: "'id' is required for 'inodes' report"})
			}
			outBaseName = fmt.Sprintf("inodes_%s_%s", req.ID, stamp)
			expectedExt = ".jpg"
			finalOutPath = filepath.Join(reportsBase, outBaseName+expectedExt)
			outputLog = Reportes.GenerarReporteInodo(req.Ruta, finalOutPath, req.ID)

		case "blocks":
			if req.ID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(genResp{Ok: false, Message: "'id' is required for 'blocks' report"})
			}
			outBaseName = fmt.Sprintf("blocks_%s_%s", req.ID, stamp)
			expectedExt = ".jpg"
			finalOutPath = filepath.Join(reportsBase, outBaseName+expectedExt)
			outputLog = Reportes.GenerarReporteBloques(req.Ruta, finalOutPath, req.ID)

		case "file":
			if req.ID == "" || req.Ruta == "" {
				return c.Status(fiber.StatusBadRequest).JSON(genResp{Ok: false, Message: "'id' and 'ruta' are required for 'file' report"})
			}
			safe := strings.ReplaceAll(filepath.Base(req.Ruta), ".", "_")
			outBaseName = fmt.Sprintf("file_%s_%s_%s", req.ID, safe, stamp)
			expectedExt = ".txt"
			finalOutPath = filepath.Join(reportsBase, outBaseName+expectedExt)
			outputLog = Reportes.GenerarReporteFile(req.Ruta, finalOutPath, req.ID)

		case "bm_blocks":
			if req.ID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(genResp{Ok: false, Message: "'id' is required for 'bm_blocks' report"})
			}
			outBaseName = fmt.Sprintf("bm_blocks_%s_%s", req.ID, stamp)
			expectedExt = ".txt"
			finalOutPath = filepath.Join(reportsBase, outBaseName+expectedExt)
			outputLog = Reportes.GenerarReporteBitmapBloques(finalOutPath, req.ID)

		case "bm_inodes":
			if req.ID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(genResp{Ok: false, Message: "'id' is required for 'bm_inodes' report"})
			}
			outBaseName = fmt.Sprintf("bm_inodes_%s_%s", req.ID, stamp)
			expectedExt = ".txt"
			finalOutPath = filepath.Join(reportsBase, outBaseName+expectedExt)
			outputLog = Reportes.GenerarReporteBitmapInodos(finalOutPath, req.ID)

		case "disk":
			if req.ID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(genResp{Ok: false, Message: "'id' is required for 'disk' report"})
			}
			mp, found := Entornos.GetMountedPartitionByID(req.ID)
			if !found {
				return c.Status(404).JSON(genResp{Ok: false, Message: "Mounted partition not found"})
			}
			outBaseName = fmt.Sprintf("disk_%s_%s", req.ID, stamp)
			expectedExt = ".pdf"
			finalOutPath = filepath.Join(reportsBase, outBaseName+expectedExt)
			outputLog = Reportes.GenerarReporteDisk(mp.MountPath, finalOutPath)

		default:
			return c.Status(fiber.StatusNotImplemented).JSON(genResp{Ok: false, Message: fmt.Sprintf("Report '%s' not supported", req.Name)})
		}

		// --- Procesar resultado + fallback DOT si falta imagen ---
		publicURL := extractS3URL(outputLog)
		resp := genResp{
			Ok:          true,
			PublicURL:   publicURL,
			ContentType: contentTypeByExt(finalOutPath),
			Message:     outputLog,
		}

		if publicURL == "" {
			cleanPath := filepath.Clean(finalOutPath)
			if inSandbox(cleanPath) {
				if _, err := os.Stat(cleanPath); err == nil {
					resp.LocalPath = cleanPath
				} else {
					// Si esperábamos .jpg/.png (como LS), intenta .dot
					dotPath := strings.TrimSuffix(cleanPath, filepath.Ext(cleanPath)) + ".dot"
					if _, err2 := os.Stat(dotPath); err2 == nil && inSandbox(dotPath) {
						resp.LocalPath = dotPath
						resp.ContentType = contentTypeByExt(dotPath)
						resp.Message += "\nNota: Graphviz no disponible; devolviendo el .dot."
					} else {
						resp.Message += "\nWarning: Report file not found locally (.jpg/.png/.dot)."
					}
				}
			} else {
				resp.Message += "\nWarning: Generated report path is outside the allowed sandbox."
			}
		}

		return c.JSON(resp)
	})

	/* ---------- Endpoint específico (opcional) para LS ---------- */
	app.Post("/reports/generate-ls", func(c *fiber.Ctx) error {
		var body struct {
			ID   string `json:"id"`             // ID de la partición montada
			Ruta string `json:"ruta,omitempty"` // ruta lógica; default "/"
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(genResp{Ok: false, Message: "JSON inválido"})
		}
		body.ID = strings.TrimSpace(body.ID)
		body.Ruta = strings.TrimSpace(body.Ruta)
		if body.ID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(genResp{Ok: false, Message: "Campo 'id' es obligatorio"})
		}
		if err := os.MkdirAll(reportsBase, 0o755); err != nil {
			return c.Status(500).JSON(genResp{Ok: false, Message: "No se pudo preparar carpeta de reportes"})
		}

		stamp := time.Now().Format("20060102_150405")
		outName := fmt.Sprintf("ls_%s_%s.jpg", body.ID, stamp)
		outPath := filepath.Join(reportsBase, outName)

		outputLog := Reportes.GenerarReporteLS(body.Ruta, outPath, body.ID)

		publicURL := extractS3URL(outputLog)
		resp := genResp{
			Ok:          true,
			PublicURL:   publicURL,
			ContentType: "image/jpeg",
			Message:     outputLog,
		}
		if publicURL == "" {
			cleanPath := filepath.Clean(outPath)
			if inSandbox(cleanPath) {
				if _, err := os.Stat(cleanPath); err == nil {
					resp.LocalPath = cleanPath
				} else {
					// Fallback al DOT si no se generó imagen
					dotPath := strings.TrimSuffix(cleanPath, filepath.Ext(cleanPath)) + ".dot"
					if _, err2 := os.Stat(dotPath); err2 == nil && inSandbox(dotPath) {
						resp.LocalPath = dotPath
						resp.ContentType = contentTypeByExt(dotPath)
						resp.Message += "\nNota: Graphviz no disponible; devolviendo el .dot."
					}
				}
			}
		}
		return c.JSON(resp)
	})

	// Proxy para servir archivos locales generados
	app.Get("/reports/proxy", func(c *fiber.Ctx) error {
		p := c.Query("path")
		if strings.TrimSpace(p) == "" {
			return c.Status(fiber.StatusBadRequest).SendString("'path' query parameter is required")
		}
		cleanPath := filepath.Clean(p)
		if !inSandbox(cleanPath) {
			return c.Status(fiber.StatusForbidden).SendString("Access denied")
		}
		if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
			return c.Status(fiber.StatusNotFound).SendString("File not found.")
		}
		return c.SendFile(cleanPath, false)
	})

	// Startup info
	base, tested := detectDisksDir("")
	if base == "" {
		fmt.Printf("⚠ No disk directory found. Searched: %v\n", tested)
	} else {
		fmt.Printf("📂 Using disk directory: %s\n", base)
	}
	fmt.Println("🚀 GoDisk Backend ready at http://localhost:3001")

	if err := app.Listen(":3001"); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}
}
