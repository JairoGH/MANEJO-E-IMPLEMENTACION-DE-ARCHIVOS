package Analizador

import (
	"backend/Estructuras"
	"flag"
	"strings"
)

// Funcion el comando mkfs.
func fn_mkfs(input string) string {
	var output strings.Builder
	fs := flag.NewFlagSet("mkfs", flag.ExitOnError)
	id := fs.String("id", "", "Id")
	type_ := fs.String("type", "full", "Tipo")
	fs_ := fs.String("fs", "2fs", "Fs")

	// Parsear la cadena de entrada.
	matches := paramRegex.FindAllStringSubmatch(input, -1)

	for _, match := range matches {
		flagName := match[1]
		flagValue := match[2]

		flagValue = strings.Trim(flagValue, "\"")

		switch flagName {
		case "id", "type", "fs":
			fs.Set(flagName, flagValue)
		default:
			output.WriteString("Error: Flag not found")
		}
	}

	// Verifica que se hayan establecido todas las flags necesarias
	if *id == "" {
		return "Error: id es un parámetro obligatorio."
	}

	// Llamar a la función
	output.WriteString(string(Estructuras.Mkfs(*id, *type_, *fs_)))
	return output.String()
}

// Funcion para procesa el comando CAT.
func fn_cat(params string) string {
	var output strings.Builder
	paramMap := ExtractParams(params)

	// Obtener solo los parámetros file1, file2, etc.
	fileParams := make(map[string]string)
	for key, value := range paramMap {
		if strings.HasPrefix(key, "file") {
			fileParams[key] = value
		}
	}

	if len(fileParams) == 0 {
		return "Error: Se requiere al menos un parámetro -file."
	}

	// Llamar a la función Cat con solo los parámetros de archivo
	output.WriteString(Estructuras.Cat(fileParams))
	return output.String()
}
