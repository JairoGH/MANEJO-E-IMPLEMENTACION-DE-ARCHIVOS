package Estructuras

import (
	"backend/Particiones"
	"fmt"
	"strings"
)

// Funcion para imprimir el contenido de un bloque de carpetas
func PrintFolderblock(folderblock Particiones.FolderBlock) string {
	var output strings.Builder
	output.WriteString(" =========================================================  \n")
	output.WriteString(" ===================  FOLDERBLOCK  =======================  \n")
	output.WriteString(" =========================================================  \n")
	for i, content := range folderblock.B_content {
		output.WriteString(fmt.Sprintf("  Content %d: Name: %s, Inodo: %d\n", i, string(content.B_name[:]), content.B_inodo))
	}
	output.WriteString(" =========================================================  \n")
	return output.String()
}

// Funcion para imprimir el contenido de un bloque de archivos
func PrintFileblock(fileblock Particiones.FileBlock) string {
	var output strings.Builder
	output.WriteString(" =========================================================  \n")
	output.WriteString(" =====================   FILEBLOCK  ======================  \n")
	output.WriteString(" =========================================================  \n")
	output.WriteString(fmt.Sprintf("  B_content: %s\n", string(fileblock.B_content[:])))
	output.WriteString(" =========================================================  \n")
	return output.String()
}
