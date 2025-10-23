package Analizador

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Utils"
	"fmt"
	"os"
	"strings"
)

// Comando: contar inodos usados del disco montado (por id)
// Uso típico desde tu parser:  fmt.Print(Reportes.PrintInodeUsage("721A"))
func PrintInodeUsage(id string) string {
	var out strings.Builder

	// 1) Buscar partición montada
	part, err := getMountedPartitionSafe(id)
	if err != nil {
		return fmt.Sprintf("Error: %v\n", err)
	}

	// 2) Abrir el archivo del disco
	f, err := Utils.OpenFile(part.MountPath)
	if err != nil {
		return fmt.Sprintf("Error al abrir archivo: %v\n", err)
	}
	defer f.Close()

	// 3) Leer superbloque (se asume en MountStart)
	sb, err := readSuperblockSafe(f, part)
	if err != nil {
		return fmt.Sprintf("Error al leer superbloque: %v\n", err)
	}

	// 4) Recorrer bitmap de inodos
	used, free, usedList, err := countInodesFromBitmap(f, sb, part, 20) // muestra hasta 20 ids usados
	if err != nil {
		return fmt.Sprintf("Error al leer bitmap de inodos: %v\n", err)
	}

	// 5) Imprimir resumen
	fmt.Fprintf(&out, "=== RESUMEN DE INODOS (id=%s) ===\n", id)
	fmt.Fprintf(&out, "Total de inodos: %d\n", sb.S_inodes_count)
	fmt.Fprintf(&out, "Inodos usados  : %d\n", used)
	fmt.Fprintf(&out, "Inodos libres  : %d\n", free)

	if len(usedList) > 0 {
		fmt.Fprintf(&out, "Algunos inodos marcados como usados: %v\n", usedList)
	}
	return out.String()
}

// -------------------- helpers internos --------------------

func countInodesFromBitmap(file *os.File, sb Particiones.SuperBlock, part Entornos.MountedPartition, sample int) (used, free int32, sampleUsed []int32, err error) {
	sampleUsed = make([]int32, 0, sample)
	bmStart := part.MountStart + sb.S_bm_inode_start

	for i := int32(0); i < sb.S_inodes_count; i++ {
		byteOff := bmStart + (i / 8)
		bitMask := byte(1 << (i % 8))

		var b [1]byte
		if e := Utils.ReadFile(file, &b, int64(byteOff)); e != nil {
			return 0, 0, nil, fmt.Errorf("fallo leyendo bitmap en inode=%d: %v", i, e)
		}
		if (b[0] & bitMask) != 0 {
			used++
			if int32(len(sampleUsed)) < int32(sample) {
				sampleUsed = append(sampleUsed, i)
			}
		} else {
			free++
		}
	}
	return
}

// ====== Reuso de tus utilidades ya definidas ======

func getMountedPartitionSafe(id string) (Entornos.MountedPartition, error) {
	if id == "" {
		return Entornos.MountedPartition{}, fmt.Errorf("ID de partición no puede estar vacío")
	}
	for _, partitions := range Entornos.GetMountedPartitions() {
		for _, p := range partitions {
			if p.MountID == id {
				return p, nil
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
	if sb.S_inodes_count <= 0 || sb.S_blocks_count <= 0 {
		return sb, fmt.Errorf("valores inválidos en superbloque")
	}
	return sb, nil
}
