package Particiones

import (
	"fmt"
	"strings"
)

type MBR struct {
	MBR_Tamano    int32
	MBR_FechaCr   [19]byte
	MBR_DiskSig   int32
	MBR_DiskFit   [1]byte
	MBR_Partition [4]Partition
}

type Partition struct {
	Part_Status      [1]byte
	Part_Type        [1]byte
	Part_Fit         [1]byte
	Part_Start       int32
	Part_Size        int32
	Part_Name        [16]byte
	Part_Correlative int32
	Part_ID          [4]byte
}

type EBR struct {
	Part_Mount byte
	Part_Fit   byte
	Part_Start int32
	Part_Size  int32
	Part_Next  int32
	Part_Name  [16]byte
}

func PrintMBR(mbr MBR) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf(" Signature: %d", mbr.MBR_DiskSig))
	output.WriteString(fmt.Sprintf("Fecha de creación: %s", string(mbr.MBR_FechaCr[:])))
	output.WriteString(fmt.Sprintf("Tamaño: %d bytes", mbr.MBR_Tamano))
	output.WriteString(fmt.Sprintf("Ajuste: %s", string(mbr.MBR_DiskFit[:])))

	return output.String()
}

func PrintPartition(part Partition) string {

	var output strings.Builder

	output.WriteString(fmt.Sprintf(" Nombre: %s", string(part.Part_Name[:])))
	output.WriteString(fmt.Sprintf(" Type: %s", string(part.Part_Type[:])))
	output.WriteString(fmt.Sprintf(" Start: %d", part.Part_Start))
	output.WriteString(fmt.Sprintf(" Size: %d", part.Part_Size))
	output.WriteString(fmt.Sprintf(" Status: %s", string(part.Part_Status[:])))
	output.WriteString(fmt.Sprintf(" ID: %s", string(part.Part_ID[:])))

	return output.String()
}

func PrintEBR(ebr EBR) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf(" Nombre: %s", string(ebr.Part_Name[:])))
	output.WriteString(fmt.Sprintf(" Fit: %c", ebr.Part_Fit))
	output.WriteString(fmt.Sprintf(" Start: %d", ebr.Part_Start))
	output.WriteString(fmt.Sprintf(" Size: %d", ebr.Part_Size))
	output.WriteString(fmt.Sprintf(" Next: %d", ebr.Part_Next))
	output.WriteString(fmt.Sprintf(" Mount: %c", ebr.Part_Mount))

	return output.String()
}
