package Analizador

import (
	"regexp"
	"strings"
)

var paramRegex = regexp.MustCompile(`-(\w+)=("[^"]+"|\S+)`)

func GetInput(input string) (string, string) {
	parts := strings.Fields(input)
	if len(parts) > 0 {
		commands := strings.ToLower(parts[0])
		params := strings.Join(parts[1:], " ")
		return commands, params
	}
	return "", input
}

func ExtractParams(params string) map[string]string {
	matches := paramRegex.FindAllStringSubmatch(params, -1)
	paramMap := make(map[string]string)

	for _, match := range matches {
		flagName := strings.ToLower(match[1])
		flagValue := strings.Trim(match[2], "\"")
		if flagName == "path" {

			flagValue = strings.ToLower(flagValue)
		}
		paramMap[flagName] = flagValue
	}

	return paramMap
}

func AnalyzerCommand(commands string, params string) string {
	ExtractParams(params)

	result := "> " + commands + " " + params + "\n"

	switch {
	case strings.Contains(commands, "mkdisk"):
		return result + fn_mkdisk(params)
	case strings.Contains(commands, "rmdisk"):
		return result + fn_rmdisk(params)
	case strings.Contains(commands, "fdisk"):
		return result + fn_fdisk(params)
	default:
		return result + "Error: Comando inválido o no encontrado"
	}
}
