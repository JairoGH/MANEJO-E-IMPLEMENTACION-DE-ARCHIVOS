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
	case strings.Contains(commands, "mounted"):
		return result + fn_mounted(params)
	case strings.Contains(commands, "mount"):
		return result + fn_mount(params)
	case strings.Contains(commands, "mkfs"):
		return result + fn_mkfs(params)
	case strings.Contains((commands), "cat"):
		return result + fn_cat(params)
	case strings.Contains(commands, "login"):
		return result + fn_login(params)
	case strings.Contains(commands, "logout"):
		return result + fn_logout(params)
	case strings.Contains(commands, "mkgrp"):
		return result + fn_mkgrp(params)
	case strings.Contains(commands, "rmgrp"):
		return result + fn_rmgrp(params)
	case strings.Contains(commands, "mkusr"):
		return result + fn_mkusr(params)
	case strings.Contains(commands, "rmusr"):
		return result + fn_rmusr(params)
	case strings.Contains(commands, "chgrp"):
		return result + fn_chgrp(params)
	case strings.Contains(commands, "mkfile"):
		return result + fn_mkfile(params)
	case strings.Contains(commands, "mkdir"):
		return result + fn_mkdir(params)
	case strings.Contains(commands, "rep"):
		return result + fn_rep(params)
	default:
		return result + "Error: Comando inválido o no encontrado"
	}
}
