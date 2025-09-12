// bm_inode_autodetect.go
package Reportes

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Utils"
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unsafe"
)

//
// ============================ Utilitarios básicos ============================
//

func ensureParentDirs(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func writeSuperblock(f *os.File, sbStart int64, sb *Particiones.SuperBlock) error {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, sb); err != nil {
		return fmt.Errorf("writeSuperblock: binary.Write: %v", err)
	}
	if _, err := f.WriteAt(buf.Bytes(), sbStart); err != nil {
		return fmt.Errorf("writeSuperblock: WriteAt: %v", err)
	}
	return nil
}

func read1ByteAt(f *os.File, off int64) (byte, error) {
	b := []byte{0}
	_, err := f.ReadAt(b, off)
	return b[0], err
}
func write1ByteAt(f *os.File, off int64, v byte) error {
	_, err := f.WriteAt([]byte{v}, off)
	return err
}

// Imprime 20 bits por línea
func writeBit(sb *strings.Builder, bit int, bitCount *int) {
	if bit == 1 {
		sb.WriteString("1")
	} else {
		sb.WriteString("0")
	}
	*bitCount++
	if *bitCount%20 == 0 {
		sb.WriteString("\n")
	} else {
		sb.WriteString(" ")
	}
}

//
// ============================ Inodo: heurística robusta ============================
//

func anyNonZero(x interface{}) bool {
	v := reflect.ValueOf(x)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() != 0
	case reflect.Array, reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if anyNonZero(v.Index(i).Interface()) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func inodeHasAnyBlock(in *Particiones.Inode) bool {
	for _, p := range in.I_block {
		// En la mayoría de implementaciones, -1 significa vacío; 0..∞ válidos
		if p >= 0 {
			return true
		}
	}
	return false
}

func inodeLooksUsedRobust(in *Particiones.Inode) bool {
	// tipo
	if len(in.I_type) > 0 && (in.I_type[0] == '0' || in.I_type[0] == '1' || in.I_type[0] != 0) {
		return true
	}
	// permisos/propietarios (sin asumir tipo exacto en el struct)
	if anyNonZero(in.I_perm) || anyNonZero(in.I_uid) || anyNonZero(in.I_gid) {
		return true
	}
	// tamaño
	if in.I_size > 0 {
		return true
	}
	// punteros
	if inodeHasAnyBlock(in) {
		return true
	}
	return false
}

//
// ============================ Orden de bits ============================
//

// Bit order
type bitOrder int

const (
	msbFirst bitOrder = iota // b7..b0
	lsbFirst                 // b0..b7
)

func getBit(b byte, idx int32, order bitOrder) int {
	if order == msbFirst {
		return int((b >> (7 - idx)) & 1)
	}
	// lsbFirst
	return int((b >> idx) & 1)
}

func setBit(b byte, idx int32, val int, order bitOrder) byte {
	var mask byte
	if order == msbFirst {
		mask = 1 << (7 - idx)
	} else {
		mask = 1 << idx
	}
	if val == 1 {
		return b | mask
	}
	return b &^ mask
}

// Auto-detecta si el bitmap usa MSB-first o LSB-first, comparando
// el primer byte del bitmap con la “verdad” que leemos de la tabla de inodos.
func autodetectBitOrder(f *os.File, bmStart int64, inodeStart int64, stride int64, sample int32) bitOrder {
	// lee primer byte del bitmap
	first, err := read1ByteAt(f, bmStart)
	if err != nil {
		// default razonable: MSB
		return msbFirst
	}
	// lee primeros 'sample' inodos reales
	matchMSB, matchLSB := 0, 0
	var ino Particiones.Inode
	for i := int32(0); i < sample; i++ {
		if err := Utils.ReadFile(f, &ino, inodeStart+int64(i)*stride); err != nil {
			continue
		}
		used := 0
		if inodeLooksUsedRobust(&ino) {
			used = 1
		}
		if getBit(first, i, msbFirst) == used {
			matchMSB++
		}
		if getBit(first, i, lsbFirst) == used {
			matchLSB++
		}
	}
	if matchLSB > matchMSB {
		return lsbFirst
	}
	return msbFirst
}

//
// ============================ Auto-detección de stride ============================
//

func sizeofInodeStruct() int64 {
	var tmp Particiones.Inode
	return int64(unsafe.Sizeof(tmp))
}

// Escanea los primeros N inodos con cada stride candidato y escoge el que
// produce más “inodos válidos” (según la heurística), con límites sanos.
func pickBestStride(f *os.File, sb *Particiones.SuperBlock, maxHeaderEnd int64) int64 {
	candidates := []int64{
		int64(sb.S_inode_size),
		sizeofInodeStruct(),
		128, 160, 192, 256, // tamaños típicos
	}

	// sanity: límites
	inodeRegion := int64(sb.S_block_start) - int64(sb.S_inode_start)
	if inodeRegion <= 0 {
		// fallback: al menos struct size
		return sizeofInodeStruct()
	}

	bestStride := int64(0)
	bestScore := -1

	sample := int32(64)
	if sb.S_inodes_count < sample {
		sample = sb.S_inodes_count
	}

	for _, stride := range candidates {
		if stride <= 0 {
			continue
		}
		if stride > inodeRegion { // stride imposible
			continue
		}
		// límite de inodos que caben con ese stride
		maxInodes := inodeRegion / stride
		if maxInodes <= 0 {
			continue
		}
		// escanear primeros 'sample'
		score := 0
		var ino Particiones.Inode
		for i := int32(0); i < sample; i++ {
			off := int64(sb.S_inode_start) + int64(i)*stride
			if off+stride > int64(sb.S_block_start) {
				break
			}
			if err := Utils.ReadFile(f, &ino, off); err != nil {
				continue
			}
			if inodeLooksUsedRobust(&ino) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestStride = stride
		}
	}

	// Si nada calza, último recurso: struct size
	if bestStride <= 0 {
		bestStride = sizeofInodeStruct()
	}
	return bestStride
}

//
// ============================ GENERADOR: BM de inodos ============================
//

// Recalcula desde la tabla de inodos, autodetecta stride y orden de bits,
// corrige bitmap + S_free_inodes_count y emite 20 registros por línea.
func GenerarReporteBitmapInodos(outputPath string, id string) string {
	// validaciones
	if strings.TrimSpace(outputPath) == "" {
		return "Error: No se especificó la ruta del archivo de reporte."
	}
	if strings.TrimSpace(id) == "" {
		return "Error: No se especificó el ID de partición (-id)."
	}

	// resolver disco por ID
	var filePath string
	var found bool
	for _, parts := range Entornos.GetMountedPartitions() {
		for _, p := range parts {
			if p.MountID == id {
				filePath = p.MountPath
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return fmt.Sprintf("Error: No se encontró la partición con ID: %s", id)
	}

	// abrir disco, leer MBR y SB
	f, err := Utils.OpenFile(filePath)
	if err != nil {
		return fmt.Sprintf("Error: No se pudo abrir el archivo en la ruta: %s", filePath)
	}
	defer f.Close()

	var mbr Particiones.MBR
	if err := Utils.ReadFile(f, &mbr, 0); err != nil {
		return "Error: No se pudo leer el MBR desde el archivo."
	}

	idx := -1
	for i := 0; i < 4; i++ {
		if mbr.MBR_Partition[i].Part_Size == 0 {
			continue
		}
		if strings.Contains(string(mbr.MBR_Partition[i].Part_ID[:]), id) {
			if mbr.MBR_Partition[i].Part_Status[0] != '1' {
				return "Error: La partición no está montada."
			}
			idx = i
			break
		}
	}
	if idx == -1 {
		return "Error: No se encontró la partición."
	}

	var sb Particiones.SuperBlock
	sbStart := int64(mbr.MBR_Partition[idx].Part_Start)
	if err := Utils.ReadFile(f, &sb, sbStart); err != nil {
		return "Error: No se pudo leer el superbloque."
	}
	if sb.S_inodes_count <= 0 {
		return "Error: Superbloeque inválido (inodos)."
	}

	// detectar stride real
	stride := pickBestStride(f, &sb, int64(sb.S_inode_start))
	if stride <= 0 {
		return "Error: No se pudo determinar el tamaño real de los inodos."
	}

	inodeStart := int64(sb.S_inode_start)
	blockStart := int64(sb.S_block_start)
	if inodeStart >= blockStart {
		return "Error: Rango de tabla de inodos inválido."
	}

	// detectar orden de bits del bitmap
	order := autodetectBitOrder(f, int64(sb.S_bm_inode_start), inodeStart, stride, 8)

	// crear archivo de salida
	if err := ensureParentDirs(outputPath); err != nil {
		return fmt.Sprintf("Error: No se pudo crear la carpeta de salida: %v", err)
	}
	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Sprintf("Error: No se pudo crear el archivo de reporte en: %s", outputPath)
	}
	defer out.Close()

	var txt strings.Builder
	txt.WriteString("═════════════════════ BITMAP INODES (AUTO) ═════════════════════════\n")

	// recorrer todos los inodos, reconstruir ocupación y autocorregir
	usedCount := int32(0)
	bitCount := 0
	total := sb.S_inodes_count
	bmStart := int64(sb.S_bm_inode_start)

	for i := int32(0); i < total; i++ {
		off := inodeStart + int64(i)*stride
		if off+stride > blockStart {
			// fuera de la tabla (sanidad)
			writeBit(&txt, 0, &bitCount)
			continue
		}

		var ino Particiones.Inode
		if err := Utils.ReadFile(f, &ino, off); err != nil {
			writeBit(&txt, 0, &bitCount)
			continue
		}

		realUsed := 0
		if inodeLooksUsedRobust(&ino) {
			realUsed = 1
		}

		// leer bit actual y corregir si difiere
		byteIndex := int64(i / 8)
		bitIndex := i % 8

		curByte, err := read1ByteAt(f, bmStart+byteIndex)
		if err != nil {
			// no pudimos leer; al menos imprimir lo real
			writeBit(&txt, realUsed, &bitCount)
			if realUsed == 1 {
				usedCount++
			}
			continue
		}

		cur := getBit(curByte, bitIndex, order)
		if cur != realUsed {
			newB := setBit(curByte, bitIndex, realUsed, order)
			if err := write1ByteAt(f, bmStart+byteIndex, newB); err == nil {
				cur = realUsed
			}
		}

		writeBit(&txt, cur, &bitCount)
		if cur == 1 {
			usedCount++
		}
	}
	if bitCount%20 != 0 {
		txt.WriteString("\n")
	}

	// actualizar SB si cambió el conteo de libres
	wantFree := sb.S_inodes_count - usedCount
	if wantFree != sb.S_free_inodes_count {
		sb.S_free_inodes_count = wantFree
		if err := writeSuperblock(f, sbStart, &sb); err != nil {
			txt.WriteString(fmt.Sprintf("\n# Aviso: no se pudo actualizar superbloque: %v\n", err))
		}
	}

	// pie
	txt.WriteString("\n# Estadísticas:\n")
	txt.WriteString(fmt.Sprintf("# Total inodos: %d\n", total))
	txt.WriteString(fmt.Sprintf("# Inodos ocupados (auto): %d\n", usedCount))
	txt.WriteString(fmt.Sprintf("# Inodos libres  (auto): %d\n", total-usedCount))
	txt.WriteString(fmt.Sprintf("# Porcentaje usado: %.6f%%\n", float64(usedCount)/float64(total)*100.0))
	txt.WriteString("# Nota: stride y orden de bits autodetectados para reflejar la tabla real.\n")

	if _, err := out.WriteString(txt.String()); err != nil {
		return "Error: No se pudo escribir en el archivo de reporte."
	}
	return fmt.Sprintf("Reporte bitmap de inodos (auto) generado: %s\nInodos ocupados: %d/%d",
		outputPath, usedCount, total)
}
