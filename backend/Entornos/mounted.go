package Entornos

import (
	"fmt"
	"strings"
)

// Mapa para almacenar las particiones montadas
var mountedPartitions = make(map[string][]MountedPartition)

// Estructura para almacenar las particiones montadas
type MountedPartition struct {
	MountPath   string
	MountName   string
	MountID     string
	MountStatus byte
	LoggedIn    bool
	MountStart  int32
}

func ImprimirMountedPartitions() string {
	var output strings.Builder
	// Si no hay particiones montadas, muestra un mensaje y termina la función.
	if len(mountedPartitions) == 0 {
		return "No hay Particiones Montadas."
	}

	// Itera sobre cada disco montado y sus particiones.
	for diskID, partitions := range mountedPartitions {
		output.WriteString(fmt.Sprintf("  Disco ID: %s\n", diskID))
		for _, partition := range partitions {
			// Determina si la partición está logueada o no.
			loginStatus := "No"
			if partition.LoggedIn {
				loginStatus = "Sí"
			}
			// Imprime los detalles de la partición.
			output.WriteString(fmt.Sprintf("  - Partición Name: %s\n", partition.MountName))
			output.WriteString(fmt.Sprintf("  ID: %s", partition.MountID))
			output.WriteString(fmt.Sprintf("  Path: %s", partition.MountPath))
			output.WriteString(fmt.Sprintf("  Status: %c", partition.MountStatus))
			output.WriteString(fmt.Sprintf("  LoggedIn: %s\n", loginStatus))
		}
	}
	output.WriteString("")
	return output.String()
}

// Función para obtener las particiones montadas
func GetMountedPartitions() map[string][]MountedPartition {
	return mountedPartitions
}

// Función para marcar una partición con inicio de sesión
func ParticionConInicioSesion(id string) string {
	var output strings.Builder
	for diskID, partitions := range mountedPartitions {
		for i, partition := range partitions {
			// Si la partición coincide con el ID buscado, se inicia sesión.
			if partition.MountID == id {
				mountedPartitions[diskID][i].LoggedIn = true
				output.WriteString(fmt.Sprintf("\t Partición con ID %s encontrada.\n", id))
				output.WriteString("\t Inicio de sesión exitoso.")
				return output.String()
			}
		}
	}
	// Si no se encuentra la partición, se muestra un mensaje de error.
	output.WriteString(fmt.Sprintf("No se encontró la partición con ID %s para marcarla con Inicio de Sesión.\n", id))
	return output.String()
}

// Función para marcar una partición como sin sesión iniciada
func ParticionSinInicioSesion(id string) string {
	mountedPartitions := GetMountedPartitions()
	var output strings.Builder

	// Buscar la partición por su ID y marcar como sin sesión iniciada
	for _, partitions := range mountedPartitions {
		for i, partition := range partitions {
			if partition.MountID == id {
				partitions[i].LoggedIn = false
				output.WriteString(fmt.Sprintf("Partición con ID %s marcada como sin inicio de sesión.\n", id))
				return output.String()
			}
		}
		return output.String()
	}

	// Si no se encuentra la partición, mostrar un mensaje
	output.WriteString(fmt.Sprintf("No se encontró la partición con ID %s para marcarla como sin inicio de sesión.\n", id))
	return output.String()
}

// Función para obtener la partición montada actualmente
func GetCurrentMountedPartition() *MountedPartition {
	for _, partitions := range mountedPartitions {
		for _, partition := range partitions {
			if partition.LoggedIn {
				return &partition
			}
		}
	}
	return nil
}

// Agrega esto en tu archivo Environment/partitions.go
func GetMountedPartitionByID(id string) (*MountedPartition, bool) {
	for _, partitions := range mountedPartitions {
		for _, partition := range partitions {
			if partition.MountID == id {
				return &partition, true
			}
		}
	}
	return nil, false
}
