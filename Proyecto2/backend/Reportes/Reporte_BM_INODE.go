package Reportes

import (
	"backend/Entornos"
	"backend/Particiones"
	"backend/Utils"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerarReporteBitmapInodos crea un archivo .txt con el bitmap de inodos,
// leyendo la estructura a nivel de BITS y formateando la salida a 20 bits por línea.
func GenerarReporteBitmapInodos(outputPath string, id string) string {
	var output strings.Builder // Usar strings.Builder para el log de salida

	// --- 1. Validaciones y obtención de la partición ---
	if strings.TrimSpace(outputPath) == "" {
		output.WriteString("Error: No se especificó la ruta del archivo de reporte.\n")
		return output.String()
	}
	// Asegurarse de que el archivo de salida tenga extensión .txt
	outputPath = ensureTXTExtension(filepath.Clean(outputPath))

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		output.WriteString(fmt.Sprintf("Error: No se pudo crear la carpeta destino: %v\n", err))
		return output.String()
	}

	mountedPartition, found := Entornos.GetMountedPartitionByID(id)
	if !found {
		output.WriteString(fmt.Sprintf("Error: No se encontró la partición con ID %s\n", id))
		return output.String()
	}

	// --- 2. Abrir el disco y leer el Superbloque ---
	file, err := Utils.OpenFile(mountedPartition.MountPath)
	if err != nil {
		output.WriteString(fmt.Sprintf("Error: No se pudo abrir el archivo en la ruta: %s\n", mountedPartition.MountPath))
		return output.String()
	}
	defer file.Close()

	var sb Particiones.SuperBlock
	if err := Utils.ReadFile(file, &sb, int64(mountedPartition.MountStart)); err != nil {
		output.WriteString("Error: No se pudo leer el superbloque desde el archivo.\n")
		return output.String()
	}

	// --- 3. Validaciones del Superbloque ---
	if sb.S_inodes_count <= 0 {
		output.WriteString("Error: El número de inodos en el superbloque es inválido.\n")
		return output.String()
	}
	if sb.S_bm_inode_start <= 0 {
		output.WriteString("Error: El puntero S_bm_inode_start del superbloque es inválido.\n")
		return output.String()
	}

	// --- 4. Leer el Bitmap de Inodos (a nivel de bits) ---
	// Calcular el tamaño correcto del bitmap en bytes.
	bitmapByteSize := (sb.S_inodes_count + 7) / 8
	bitmap := make([]byte, bitmapByteSize)
	if err := Utils.ReadFile(file, &bitmap, int64(sb.S_bm_inode_start)); err != nil {
		output.WriteString("Error: No se pudo leer el bitmap de inodos.\n")
		return output.String()
	}

	// --- 5. Generar el contenido del reporte ---
	var builder strings.Builder // Este builder es para el *contenido* del .txt
	for i := int32(0); i < sb.S_inodes_count; i++ {
		byteIndex := i / 8
		bitIndex := i % 8

		// Verificar si el bit en la posición 'i' está encendido (1) o apagado (0)
		if (bitmap[byteIndex] & (1 << bitIndex)) != 0 {
			builder.WriteString("1")
		} else {
			builder.WriteString("0")
		}

		// Formatear la salida con 20 registros por línea
		if (i+1)%20 == 0 {
			builder.WriteString("\n")
		} else if i < sb.S_inodes_count-1 { // Evitar un espacio extra al final
			builder.WriteString(" ")
		}
	}
	// Añadir un salto de línea final si el último grupo no completó los 20
	if sb.S_inodes_count%20 != 0 {
		builder.WriteString("\n")
	}

	// --- 6. Escribir el reporte en el archivo ---
	err = os.WriteFile(outputPath, []byte(builder.String()), 0644)
	if err != nil {
		output.WriteString(fmt.Sprintf("Error: No se pudo escribir en el archivo de reporte: %v\n", err))
		return output.String()
	}

	output.WriteString(fmt.Sprintf("Reporte bitmap de inodos (.txt) generado localmente en: %s\n", outputPath))

	// --- 7. Subir a S3 ---
	bucketName := "proyecto2-front"
	reportS3Key := "reports/" + filepath.Base(outputPath) // outputPath es el .txt

	publicURL, errS3 := Utils.UploadS3(bucketName, outputPath, reportS3Key)
	if errS3 != nil {
		output.WriteString(fmt.Sprintf("Error al subir el TXT a S3: %v\n", errS3))
	} else {
		output.WriteString(fmt.Sprintf("TXT subido a S3 exitosamente.\n"))
		output.WriteString(fmt.Sprintf("URL Pública: %s\n", publicURL))
	}

	return output.String()
}

// ensureTXTExtension garantiza que la ruta termine en .txt
func ensureTXTExtension(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".txt" {
		return path
	}
	return strings.TrimSuffix(path, ext) + ".txt"
}
