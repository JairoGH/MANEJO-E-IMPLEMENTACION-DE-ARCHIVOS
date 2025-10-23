package Analizador

import (
	"regexp"
	"strings"
)

// Go/RE2 no soporta lookahead (?=) ni grupos no capturantes (?:)
// Usamos dos regex simples y seguras:
//
//  1. Flags con valor: -key="con espacios"  ó  -key=valor
//     Nota: [^-[:space:]]+ = cualquier cosa que no sea espacio ni '-' (corta antes del siguiente flag)
var paramWithValue = regexp.MustCompile(`-(\w+)=("([^"]*)"|\S+)`)

//  2. Flags sin valor: -p  -debug
//     Matchea cuando hay espacio/INICIO antes y espacio/FIN después.
//     OJO: no matchea -key=valor (que lo toma paramWithValue).
var paramFlagOnly = regexp.MustCompile(`(^|[[:space:]])-(\w+)([[:space:]]|$)`)

func GetInput(input string) (string, string) {
	parts := strings.Fields(input)
	if len(parts) > 0 {
		commands := strings.ToLower(parts[0])
		params := strings.Join(parts[1:], " ")
		return commands, params
	}
	return "", input
}

// Solo minúsculas para la KEY; el VALOR se preserva (case-sensitive).
// Soporta valores entrecomillados y flags sin valor.
func ExtractParams(s string) map[string]string {
	m := map[string]string{}

	// 1) -key=valor
	with := paramWithValue.FindAllStringSubmatch(s, -1)
	for _, g := range with {
		key := strings.ToLower(g[1])
		val := g[2]
		// quitar comillas "..."
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		m[key] = val // preserva mayúsculas del valor (p. ej. path)
	}

	// 2) -flag (sin valor). Evitamos pisar los que ya tienen valor.
	flags := paramFlagOnly.FindAllStringSubmatch(s, -1)
	for _, g := range flags {
		key := strings.ToLower(g[2]) // el nombre del flag es el grupo 2
		if _, exists := m[key]; !exists {
			m[key] = "" // presente sin valor
		}
	}
	return m
}

func AnalyzerCommand(commands string, params string) string {
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
	case strings.Contains(commands, "inodos"):
		return result + fn_inodos(params)
	default:
		return result + "Error: Comando inválido o no encontrado"
	}
}
