package Reportes

import (
	"backend/Particiones"
	"backend/Utils"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ===================== PÚBLICO ====================

func GenerarReporteMBR(diskPath, reportPath string) string {
	var output strings.Builder

	// Abrir el archivo binario del disco montado
	file, err := Utils.OpenFile(diskPath)
	if err != nil {
		return fmt.Sprintf("Error: No se pudo abrir el archivo en la ruta: %s", diskPath)
	}
	defer file.Close()

	// Leer el objeto MBR desde el archivo binario
	var TempMBR Particiones.MBR
	if err := Utils.ReadFile(file, &TempMBR, 0); err != nil {
		return "Error: No se pudo leer el MBR desde el archivo"
	}

	// Leer y procesar los EBRs si hay particiones extendidas (recolección opcional)
	var ebrs []Particiones.EBR
	for i := 0; i < 4; i++ {
		if strings.ToLower(trimC(TempMBR.MBR_Partition[i].Part_Type[:])) == "e" { // Partición extendida
			ebrPosition := TempMBR.MBR_Partition[i].Part_Start
			guard := 0
			for ebrPosition != -1 && guard < 1000 { // guard por seguridad
				var tempEBR Particiones.EBR
				if err := Utils.ReadFile(file, &tempEBR, int64(ebrPosition)); err != nil {
					output.WriteString("Error: No se pudo leer el EBR desde el archivo\n")
					break
				}
				ebrs = append(ebrs, tempEBR)
				ebrPosition = tempEBR.Part_Next
				guard++
			}
		}
	}

	// Asegurar extensión de imagen y detectar formato
	outImgPath, format := ensureImagePath(reportPath)

	// Generar el archivo .dot del MBR con EBRs
	if err := GenerateMBRReport(TempMBR, ebrs, outImgPath, file); err != nil {
		output.WriteString(fmt.Sprintf("Error al generar el reporte MBR: %v", err))
		return output.String()
	}

	output.WriteString(fmt.Sprintf("Reporte MBR (.dot) generado exitosamente para: %s\n", outImgPath))

	// Renderizar el archivo .dot a imagen usando Graphviz
	dotFile := strings.TrimSuffix(outImgPath, filepath.Ext(outImgPath)) + ".dot"

	// Verificar disponibilidad de 'dot'
	if _, err := exec.LookPath("dot"); err != nil {
		output.WriteString("Advertencia: Graphviz no está instalado o 'dot' no está en PATH.\n")
		output.WriteString(fmt.Sprintf("Dejé el .dot en: %s\n", dotFile))
		output.WriteString("Puedes renderizar manualmente con: dot -T" + format + " " + dotFile + " -o " + outImgPath + "\n")
		return output.String()
	}

	cmd := exec.Command("dot", "-T"+format, dotFile, "-o", outImgPath)
	if err := cmd.Run(); err != nil {
		output.WriteString(fmt.Sprintf("Error al renderizar el archivo .dot a imagen: %v\n", err))
		output.WriteString(fmt.Sprintf("Puedes intentar manualmente: dot -T%s %s -o %s\n", format, dotFile, outImgPath))
		return output.String()
	}

	output.WriteString(fmt.Sprintf("Imagen generada exitosamente en: %s\n", outImgPath))
	return output.String()
}

// ===================== PRIVADO =====================

// GenerateMBRReport genera un reporte del MBR y los EBRs en formato .dot.
// Nota: outputPath se utiliza para derivar el .dot (misma base, extensión .dot).
func GenerateMBRReport(mbr Particiones.MBR, ebrs []Particiones.EBR, outputPath string, file *os.File) error {
	reportsDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(reportsDir, os.ModePerm); err != nil {
		return fmt.Errorf("error al crear la carpeta de reportes: %v", err)
	}

	// Crear el archivo .dot donde se generará el reporte
	dotFilePath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dot"
	dotFile, err := os.Create(dotFilePath)
	if err != nil {
		return fmt.Errorf("error al crear el archivo .dot de reporte: %v", err)
	}
	defer dotFile.Close()

	// Iniciar el contenido del archivo en formato Graphviz
	var graphContent strings.Builder
	graphContent.WriteString("digraph G {\n")
	graphContent.WriteString("\tnode [fillcolor=lightyellow style=filled]\n")
	graphContent.WriteString("\trankdir=LR;\n")

	// Crear una sola tabla para el MBR y las particiones
	graphContent.WriteString("\tsubgraph cluster_MBR {\n")
	graphContent.WriteString("\t\tcolor=lightblue fillcolor=aliceblue label=\"MBR Report\" style=filled\n")
	graphContent.WriteString("\t\tmbr [label=<<table border=\"0\" cellborder=\"1\" cellspacing=\"0\" cellpadding=\"4\">\n")

	// Encabezado de la tabla
	graphContent.WriteString("\t\t\t<tr><td colspan=\"2\" bgcolor=\"cadetblue\"><b>MBR Information</b></td></tr>\n")
	graphContent.WriteString(fmt.Sprintf("\t\t\t<tr><td><b>Tamaño</b></td><td>%d</td></tr>\n", mbr.MBR_Tamano))
	graphContent.WriteString(fmt.Sprintf("\t\t\t<tr><td><b>Fecha Creación</b></td><td>%s</td></tr>\n", trimC(mbr.MBR_FechaCr[:]))) // quita \x00
	graphContent.WriteString(fmt.Sprintf("\t\t\t<tr><td><b>Disk Signature</b></td><td>%d</td></tr>\n", mbr.MBR_DiskSig))

	// Iterar sobre las 4 particiones del MBR
	for i := 0; i < 4; i++ {
		partition := mbr.MBR_Partition[i]
		if partition.Part_Size <= 0 {
			continue
		}

		pType := strings.ToLower(trimC(partition.Part_Type[:]))
		partitionName := trimC(partition.Part_Name[:])

		// Determinar el color del encabezado
		headerColor := "white"
		switch pType {
		case "p":
			headerColor = "lightgreen"
		case "e":
			headerColor = "lightblue"
		case "l":
			headerColor = "lightyellow"
		}

		graphContent.WriteString(fmt.Sprintf("\t\t\t<tr><td colspan=\"2\" bgcolor=\"%s\"><b>Partición %d</b></td></tr>\n", headerColor, i+1))
		graphContent.WriteString(fmt.Sprintf("\t\t\t<tr><td><b>Status</b></td><td>%s</td></tr>\n", trimC(partition.Part_Status[:])))
		graphContent.WriteString(fmt.Sprintf("\t\t\t<tr><td><b>Type</b></td><td>%s</td></tr>\n", pType))
		graphContent.WriteString(fmt.Sprintf("\t\t\t<tr><td><b>Fit</b></td><td>%s</td></tr>\n", trimC(partition.Part_Fit[:])))
		graphContent.WriteString(fmt.Sprintf("\t\t\t<tr><td><b>Start</b></td><td>%d</td></tr>\n", partition.Part_Start))
		graphContent.WriteString(fmt.Sprintf("\t\t\t<tr><td><b>Size</b></td><td>%d</td></tr>\n", partition.Part_Size))
		graphContent.WriteString(fmt.Sprintf("\t\t\t<tr><td><b>Name</b></td><td>%s</td></tr>\n", partitionName))

		// Manejar particiones extendidas y sus EBRs
		if pType == "e" {
			graphContent.WriteString(fmt.Sprintf("\t\t\t<tr><td colspan=\"2\" bgcolor=\"lightpink\"><b>EBRs de la Partición Extendida %d</b></td></tr>\n", i+1))

			// Leer los EBRs desde disco (fuente de verdad) por si no se pasaron
			ebrPosition := partition.Part_Start
			guard := 0
			for {
				var ebr Particiones.EBR
				if err := Utils.ReadFile(file, &ebr, int64(ebrPosition)); err != nil {
					break
				}

				ebrName := trimC(ebr.Part_Name[:])
				graphContent.WriteString(fmt.Sprintf("\t\t\t<tr><td><b>EBR Start</b></td><td>%d</td></tr>\n", ebr.Part_Start))
				graphContent.WriteString(fmt.Sprintf("\t\t\t<tr><td><b>EBR Size</b></td><td>%d</td></tr>\n", ebr.Part_Size))
				graphContent.WriteString(fmt.Sprintf("\t\t\t<tr><td><b>EBR Next</b></td><td>%d</td></tr>\n", ebr.Part_Next))
				graphContent.WriteString(fmt.Sprintf("\t\t\t<tr><td><b>EBR Name</b></td><td>%s</td></tr>\n", ebrName))

				if ebr.Part_Next == -1 || guard > 1000 {
					break
				}
				ebrPosition = ebr.Part_Next
				guard++
			}
		}
	}

	graphContent.WriteString("\t\t</table>> shape=plaintext]\n")
	graphContent.WriteString("\t}\n")
	graphContent.WriteString("}\n")

	// Escribir el contenido en el archivo .dot
	if _, err := dotFile.WriteString(graphContent.String()); err != nil {
		return fmt.Errorf("error al escribir en el archivo .dot: %v", err)
	}

	return nil
}

// ===================== UTILIDAD =====================

func trimC(b []byte) string {
	return strings.TrimRight(string(b), "\x00 ")
}

// ensureImagePath garantiza que haya extensión válida y devuelve (rutaFinal, formatoGraphviz)
func ensureImagePath(path string) (string, string) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return path, "png"
	case ".jpg", ".jpeg":
		return path, "jpg"
	default:
		// sin extensión → forzar png
		return path + ".png", "png"
	}
}
