package Reportes

import (
	"backend/Particiones"
	"backend/Utils"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func GenerarReporteDisk(diskPath, reportPath string) string {
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

	// Leer y procesar los EBRs si hay particiones extendidas
	var ebrs []Particiones.EBR
	for i := 0; i < 4; i++ {
		if string(TempMBR.MBR_Partition[i].Part_Type[:]) == "e" { // Partición extendida
			ebrPosition := TempMBR.MBR_Partition[i].Part_Start
			guard := 0
			for ebrPosition != -1 && guard < 1000 {
				var tempEBR Particiones.EBR
				if err := Utils.ReadFile(file, &tempEBR, int64(ebrPosition)); err != nil {
					break
				}
				ebrs = append(ebrs, tempEBR)
				ebrPosition = tempEBR.Part_Next
				guard++
			}
		}
	}

	// Calcular el tamaño total del disco (evitar div/0)
	totalDiskSize := TempMBR.MBR_Tamano
	if totalDiskSize <= 0 {
		return "Error: Tamaño de disco (MBR.MBR_Tamano) inválido o cero"
	}

	// Generar el archivo .dot del DISK
	if err := GenerateDiskReport(TempMBR, ebrs, reportPath, file, totalDiskSize, filepath.Base(diskPath)); err != nil {
		output.WriteString(fmt.Sprintf("Error al generar el reporte DISK: %v", err))
		return output.String()
	}

	output.WriteString(fmt.Sprintf("Reporte DISK (.dot) generado correctamente para: %s\n", reportPath))

	// Renderizar el archivo .dot a .pdf usando Graphviz
	dotFile := strings.TrimSuffix(reportPath, filepath.Ext(reportPath)) + ".dot"
	outputPdf := strings.TrimSuffix(reportPath, filepath.Ext(reportPath)) + ".pdf"

	// Verificar 'dot' disponible
	if _, err := exec.LookPath("dot"); err != nil {
		output.WriteString("Advertencia: Graphviz no está instalado o 'dot' no está en PATH.\n")
		output.WriteString(fmt.Sprintf("Dejé el .dot en: %s\n", dotFile))
		output.WriteString(fmt.Sprintf("Para generar PDF: dot -Tpdf %s -o %s\n", dotFile, outputPdf))
		return output.String()
	}

	cmd := exec.Command("dot", "-Tpdf", dotFile, "-o", outputPdf)
	if err := cmd.Run(); err != nil {
		output.WriteString(fmt.Sprintf("Error al convertir DOT a PDF: %v\n", err))
		output.WriteString(fmt.Sprintf("Intenta manualmente: dot -Tpdf %s -o %s\n", dotFile, outputPdf))
		return output.String()
	}

	output.WriteString(fmt.Sprintf("PDF generado exitosamente en: %s\n", outputPdf))
	return output.String()
}

// ===================== GENERADOR .DOT =====================

func GenerateDiskReport(mbr Particiones.MBR, ebrs []Particiones.EBR, outputPath string, file *os.File, totalDiskSize int32, diskLabel string) error {
	reportsDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(reportsDir, os.ModePerm); err != nil {
		return fmt.Errorf("error al crear la carpeta de reportes: %v", err)
	}

	// Crear el archivo .dot
	dotFilePath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".dot"
	dotFile, err := os.Create(dotFilePath)
	if err != nil {
		return fmt.Errorf("error al crear el archivo .dot de reporte: %v", err)
	}
	defer dotFile.Close()

	// Calcular tamaño real del MBR
	mbrSize := int32(binary.Size(mbr))
	if mbrSize < 0 {
		mbrSize = 0
	}

	// Iniciar el contenido del archivo en formato Graphviz (.dot)
	var graphContent strings.Builder
	graphContent.WriteString("digraph G {\n")
	graphContent.WriteString("\tnode [shape=plaintext];\n") // <- importante para labels HTML
	graphContent.WriteString("\tgraph [splines=false];\n")
	graphContent.WriteString("\trankdir=LR;\n")

	// Título del disco con estilo
	if diskLabel == "" {
		diskLabel = "DISK"
	}
	graphContent.WriteString("\tsubgraph cluster_disk {\n")
	graphContent.WriteString(fmt.Sprintf("\t\tlabel=\"%s Report\";\n", diskLabel))
	graphContent.WriteString("\t\tstyle=\"rounded,filled\";\n")
	graphContent.WriteString("\t\tfillcolor=\"#bdc3c7\";\n")
	graphContent.WriteString("\t\tcolor=black;\n")
	graphContent.WriteString("\t\tfontcolor=black;\n")
	graphContent.WriteString("\t\tfontsize=20;\n")

	// Iniciar tabla para las particiones con estilos
	graphContent.WriteString("\t\tdisk [shape=plaintext label=<\n")
	graphContent.WriteString("\t\t\t<TABLE BORDER=\"0\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"6\">\n")
	graphContent.WriteString("\t\t\t\t<TR>\n")

	// 1. MBR con color azul
	mbrPercentage := (float64(mbrSize) / float64(totalDiskSize)) * 100.0
	graphContent.WriteString(fmt.Sprintf(
		"\t\t\t\t\t<TD BORDER=\"1\" BGCOLOR=\"#3498db\"><FONT COLOR=\"white\" POINT-SIZE=\"14\"><B>MBR</B></FONT><BR/><FONT POINT-SIZE=\"12\">%d bytes</FONT><BR/><FONT POINT-SIZE=\"12\">%.2f%%</FONT></TD>\n",
		mbrSize, mbrPercentage,
	))

	// Variables para el espacio utilizado
	usedSpace := mbrSize
	var extendedSpace int32 = 0

	// Procesar las particiones primarias y extendidas
	for i := 0; i < 4; i++ {
		partition := mbr.MBR_Partition[i]
		if partition.Part_Size <= 0 {
			continue
		}

		percentage := (float64(partition.Part_Size) / float64(totalDiskSize)) * 100.0
		partitionName := strings.TrimRight(string(partition.Part_Name[:]), "\x00")

		if string(partition.Part_Type[:]) == "p" { // Primaria
			graphContent.WriteString(fmt.Sprintf(
				"\t\t\t\t\t<TD BORDER=\"1\" BGCOLOR=\"#2ecc71\"><FONT COLOR=\"white\" POINT-SIZE=\"14\"><B>Primaria</B></FONT><BR/><FONT POINT-SIZE=\"12\">%s</FONT><BR/><FONT POINT-SIZE=\"12\">%d bytes</FONT><BR/><FONT POINT-SIZE=\"12\">%.2f%%</FONT></TD>\n",
				partitionName, partition.Part_Size, percentage,
			))
			usedSpace += partition.Part_Size

		} else if string(partition.Part_Type[:]) == "e" { // Extendida
			extendedSpace = partition.Part_Size

			graphContent.WriteString("\t\t\t\t\t<TD BORDER=\"1\" BGCOLOR=\"#f39c12\">\n")
			graphContent.WriteString("\t\t\t\t\t\t<TABLE BORDER=\"0\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"6\">\n")
			graphContent.WriteString(fmt.Sprintf("\t\t\t\t\t\t\t<TR><TD COLSPAN=\"3\" BGCOLOR=\"#f39c12\"><FONT COLOR=\"white\" POINT-SIZE=\"14\"><B>Extendida</B></FONT><BR/><FONT POINT-SIZE=\"12\">%d bytes</FONT><BR/><FONT POINT-SIZE=\"12\">%.2f%%</FONT></TD></TR>\n", extendedSpace, percentage))
			graphContent.WriteString("\t\t\t\t\t\t\t<TR>\n")

			// Procesar las particiones lógicas dentro de la extendida
			var ebrSpace int32 = 0
			for _, ebr := range ebrs {
				if ebr.Part_Size <= 0 {
					continue
				}
				logicalPercentage := (float64(ebr.Part_Size) / float64(totalDiskSize)) * 100.0
				ebrSize := int32(binary.Size(ebr))
				if ebrSize < 0 {
					ebrSize = 0
				}
				ebrPercentage := (float64(ebrSize) / float64(totalDiskSize)) * 100.0

				// EBR
				graphContent.WriteString(fmt.Sprintf(
					"\t\t\t\t\t\t\t\t<TD BORDER=\"1\" BGCOLOR=\"#e67e22\"><FONT COLOR=\"white\" POINT-SIZE=\"14\"><B>EBR</B></FONT><BR/><FONT POINT-SIZE=\"12\">%d bytes</FONT><BR/><FONT POINT-SIZE=\"12\">%.2f%%</FONT></TD>\n",
					ebrSize, ebrPercentage,
				))

				// Lógica
				graphContent.WriteString(fmt.Sprintf(
					"\t\t\t\t\t\t\t\t<TD BORDER=\"1\" BGCOLOR=\"#e74c3c\"><FONT COLOR=\"white\" POINT-SIZE=\"14\"><B>Lógica</B></FONT><BR/><FONT POINT-SIZE=\"12\">%s</FONT><BR/><FONT POINT-SIZE=\"12\">%d bytes</FONT><BR/><FONT POINT-SIZE=\"12\">%.2f%%</FONT></TD>\n",
					strings.TrimRight(string(ebr.Part_Name[:]), "\x00"), ebr.Part_Size, logicalPercentage,
				))

				ebrSpace += ebr.Part_Size + ebrSize
			}

			// Espacio libre dentro de la extendida
			freeExtended := extendedSpace - ebrSpace
			if freeExtended > 0 {
				freeExtendedPercentage := (float64(freeExtended) / float64(totalDiskSize)) * 100.0
				graphContent.WriteString(fmt.Sprintf(
					"\t\t\t\t\t\t\t\t<TD BORDER=\"1\" BGCOLOR=\"#ecf0f1\"><FONT POINT-SIZE=\"14\"><B>Libre</B></FONT><BR/><FONT POINT-SIZE=\"12\">%d bytes</FONT><BR/><FONT POINT-SIZE=\"12\">%.2f%%</FONT></TD>\n",
					freeExtended, freeExtendedPercentage,
				))
			}

			graphContent.WriteString("\t\t\t\t\t\t\t</TR>\n")
			graphContent.WriteString("\t\t\t\t\t\t</TABLE>\n")
			graphContent.WriteString("\t\t\t\t\t</TD>\n")

			usedSpace += extendedSpace
		}
	}

	// Espacio libre fuera de las particiones
	freeSpace := totalDiskSize - usedSpace
	if freeSpace > 0 {
		freePercentage := (float64(freeSpace) / float64(totalDiskSize)) * 100.0
		graphContent.WriteString(fmt.Sprintf(
			"\t\t\t\t\t<TD BORDER=\"1\" BGCOLOR=\"#ecf0f1\"><FONT POINT-SIZE=\"14\"><B>Libre</B></FONT><BR/><FONT POINT-SIZE=\"12\">%d bytes</FONT><BR/><FONT POINT-SIZE=\"12\">%.2f%%</FONT></TD>\n",
			freeSpace, freePercentage,
		))
	}

	graphContent.WriteString("\t\t\t\t</TR>\n")
	graphContent.WriteString("\t\t\t</TABLE>\n")
	graphContent.WriteString("\t\t>];\n")
	graphContent.WriteString("\t}\n")
	graphContent.WriteString("}\n")

	// Escribir el contenido en el archivo .dot
	if _, err := dotFile.WriteString(graphContent.String()); err != nil {
		return fmt.Errorf("error al escribir en el archivo .dot: %v", err)
	}

	return nil
}
