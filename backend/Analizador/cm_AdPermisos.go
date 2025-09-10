package Analizador

import (
	"backend/AdmPermisos"
	"strings"
)

// Archvivos Y permisos
func fn_mkfile(parametros string) string {
	var output strings.Builder
	// Extraer los parámetros en formato map[string]string
	paramsMap := ExtractParams(parametros)

	// Pasar los parámetros a la función Mkfile
	output.WriteString(AdmPermisos.Mkfile(paramsMap))
	return output.String()
}

// Archivos y permisos
func fn_mkdir(parametros string) string {
	var output strings.Builder
	paramsMap := ExtractParams(parametros)

	//Pasar los parámetros a la función Mkdir
	output.WriteString(AdmPermisos.Mkdir(paramsMap))
	return output.String()
}
