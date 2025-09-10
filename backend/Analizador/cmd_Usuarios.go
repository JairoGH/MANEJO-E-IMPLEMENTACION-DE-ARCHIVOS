package Analizador

import (
	"backend/Usuarios"
	"flag"
	"os"
	"strings"
)

// Funcion para procesar el comando LOGIN (Iniciar Sesion
func fn_login(input string) string {
	var output strings.Builder
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	user := fs.String("user", "", "Usuario")
	pass := fs.String("pass", "", "Contraseña")
	id := fs.String("id", "", "Id")

	fs.Parse(os.Args[1:])
	matches := paramRegex.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		flagName := match[1]
		flagValue := match[2]

		flagValue = strings.Trim(flagValue, "\"")

		switch flagName {
		case "user", "pass", "id":
			fs.Set(flagName, flagValue)
		default:
			output.WriteString(" Error: Flag not found ")
		}
	}

	output.WriteString(Usuarios.Login(*user, *pass, *id))
	return output.String()
}

// Funcion para Cerrar Sesion
func fn_logout(_ string) string {
	var output strings.Builder
	output.WriteString(Usuarios.Logout())
	return output.String()
}

// Parametro para Crear un Grupo
func fn_mkgrp(parametros string) string {
	var output strings.Builder
	params := ExtractParams(parametros)
	if name, ok := params["name"]; ok {

		output.WriteString(Usuarios.MKGRP(name))
	} else {
		output.WriteString(" Error: Falta el parámetro -name")
	}
	return output.String()
}

// Parametro para Eliminar un Grupo
func fn_rmgrp(parametros string) string {
	var output strings.Builder
	params := ExtractParams(parametros)
	if name, ok := params["name"]; ok {

		output.WriteString(Usuarios.RMGRP(name))
	} else {
		output.WriteString(" Error: Falta el parámetro -name ")
	}
	return output.String()
}

// Parametro para Crear un Usuario
func fn_mkusr(parametros string) string {
	var output strings.Builder
	params := ExtractParams(parametros)

	// Extraer los parámetros necesarios
	user, userOk := params["user"]
	pass, passOk := params["pass"]
	grp, grpOk := params["grp"]

	if !userOk || !passOk || !grpOk {
		output.WriteString(" Error: Faltan parámetros. Se requieren -user, -pass y -grp ")
		return output.String()
	}

	output.WriteString(Usuarios.MKUSR(user, pass, grp))
	return output.String()
}

// Parametro para Eliminar un Usuario
func fn_rmusr(parametros string) string {
	var output strings.Builder
	paramMap := ExtractParams(parametros)

	// Validar que exista el parámetro obligatorio
	user, userOK := paramMap["user"]

	if !userOK {
		return " Error: Falta el parámetro obligatorio (-user)"
	}

	output.WriteString(Usuarios.RMUSR(user))
	return output.String()
}

// Parametro para Cambiar el Grupo de un Usuario
func fn_chgrp(parametros string) string {
	var output strings.Builder
	paramMap := ExtractParams(parametros)

	// Validar que existan los parámetros obligatorios
	user, userOK := paramMap["user"]
	grp, grpOK := paramMap["grp"]

	if !userOK || !grpOK {
		return " Error: Faltan los parámetros obligatorios (-user, -pass y -grp)"
	}

	output.WriteString(Usuarios.CHGRP(user, grp))
	return output.String()
}
