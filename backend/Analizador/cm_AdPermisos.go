package Analizador

import (
	"backend/AdmPermisos"
	"strings"
)

// Archivos y permisos - MKFILE
func fn_mkfile(parametros string) string {
	var output strings.Builder
	paramsMap := ExtractParams(parametros)
	output.WriteString(AdmPermisos.Mkfile(paramsMap))
	return output.String()
}

// Archivos y permisos - MKDIR
func fn_mkdir(parametros string) string {
	var output strings.Builder
	paramsMap := ExtractParams(parametros)
	output.WriteString(AdmPermisos.Mkdir(paramsMap))
	return output.String()
}
