package Entornos

import (
	"backend/Particiones"
	"backend/Utils"
	"bytes"
	"fmt"
	"strings"
)

func Mount(path string, name string) string {
	var output strings.Builder
	output.WriteString("|============================================================|\n")
	output.WriteString("|======================= INICIO MOUNT ======================|\n")
	output.WriteString("|============================================================|\n")
	file, err := Utils.OpenFile(path)
	if err != nil {
		return fmt.Sprintf("  Error: No se pudo abrir el archivo en la ruta: %s\n|======================= FIN MOUNT ======================|\n", path)
	}
	defer file.Close()

	var TempMBR Particiones.MBR
	if err := Utils.ReadFile(file, &TempMBR, 0); err != nil {
		return "Error: No se pudo leer el MBR desde el archivo\n|======================= FIN MOUNT ======================|\n"
	}

	output.WriteString(fmt.Sprintf("  Buscando partición con nombre: '%s'\n", name))

	partitionFound := false
	var partition Particiones.Partition
	var partitionIndex int

	// Convertir el nombre a comparar a un arreglo de bytes de longitud fija
	nameBytes := [16]byte{}
	copy(nameBytes[:], []byte(name))

	for i := 0; i < 4; i++ {
		if TempMBR.MBR_Partition[i].Part_Type[0] == 'p' && bytes.Equal(TempMBR.MBR_Partition[i].Part_Name[:], nameBytes[:]) {
			partition = TempMBR.MBR_Partition[i]
			partitionIndex = i
			partitionFound = true
			break
		}
	}

	if !partitionFound {
		return "Error: Partición no encontrada o no es una partición primaria\n|======================= FIN MOUNT ======================|\n"
	}

	// Verificar si la partición ya está montada
	if partition.Part_Status[0] == '1' {
		return "Error: La partición ya está montada\n|======================= FIN MOUNT ======================|\n"
	}

	output.WriteString(fmt.Sprintf("  Partición encontrada: '%s' en posición %d\n", strings.TrimSpace(string(partition.Part_Name[:])), partitionIndex+1))

	// Generar el ID de la partición utilizando la función `generateDiskID`
	diskID := generateDiskID(path)
	mountedPartitionsInDisk := mountedPartitions[diskID]
	var letter byte

	if len(mountedPartitionsInDisk) == 0 {
		if len(mountedPartitions) == 0 {
			letter = 'A'
		} else {
			lastDiskID := getLastDiskID()
			lastLetter := mountedPartitions[lastDiskID][0].MountID[len(mountedPartitions[lastDiskID][0].MountID)-1]
			letter = lastLetter + 1
		}
	} else {
		letter = mountedPartitionsInDisk[0].MountID[len(mountedPartitionsInDisk[0].MountID)-1]
	}

	// Crear el ID de la partición utilizando el último par de dígitos de un carnet
	carnet := "201902672"
	lastTwoDigits := carnet[len(carnet)-2:]
	partitionID := fmt.Sprintf("%s%d%c", lastTwoDigits, partitionIndex+1, letter)

	partition.Part_Status[0] = '1'
	copy(partition.Part_ID[:], partitionID)
	TempMBR.MBR_Partition[partitionIndex] = partition

	// Almacenar la posición inicial de la partición
	mountedPartitions[diskID] = append(mountedPartitions[diskID], MountedPartition{
		MountPath:   path,
		MountName:   name,
		MountID:     partitionID,
		MountStatus: '1',
		MountStart:  partition.Part_Start,
	})

	// Escribir el MBR actualizado en el archivo
	if err := Utils.WriteFile(file, TempMBR, 0); err != nil {
		return "Error: No se pudo sobrescribir el MBR en el archivo\n|======================= FIN MOUNT ======================|\n"
	}

	// Imprimir el mensaje confirmando que la partición ha sido montada, junto con su ID.
	output.WriteString(fmt.Sprintf("  Partición montada con ID: %s\n", partitionID))

	output.WriteString("\n  MBR actualizado:\n")
	output.WriteString(Particiones.PrintMBR(TempMBR))
	output.WriteString("\n\n  Particiones Montadas:\n")
	output.WriteString(ImprimirMountedPartitions())

	output.WriteString("|========================================================|\n")
	output.WriteString("|======================= FIN MOUNT ======================|\n")
	output.WriteString("|========================================================|\n")
	return output.String()
}

func getLastDiskID() string {
	var lastDiskID string
	for diskID := range mountedPartitions {
		lastDiskID = diskID
	}
	return lastDiskID
}

func generateDiskID(path string) string {
	return strings.ToLower(path)
}
