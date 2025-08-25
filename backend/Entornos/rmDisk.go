package Entornos

import (
	"fmt"
	"os"
	"strings"
)

func RmDisk(path string) string {

	var output strings.Builder

	output.WriteString("|============================================================|\n")
	output.WriteString("|======================= INICIO RMDISK ======================|\n")
	output.WriteString("|============================================================|\n")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Sprintf("Error: El disco no existe en la ruta especificada: %s\n", path)
	}

	err := os.Remove(path)
	if err != nil {
		return fmt.Sprintf("Error: El archivo no existe en la ruta: %s: %v\n", path, err)
	}

	output.WriteString(fmt.Sprintf("Disco eliminado exitosamente en la ruta: %s\n", path))

	output.WriteString("|==============================================================|\n")
	output.WriteString("|======================== FIN RMDISK ==========================|\n")
	output.WriteString("|==============================================================|\n")

	return output.String()
}
