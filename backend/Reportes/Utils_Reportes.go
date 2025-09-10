package Reportes

import (
	"path/filepath"
	"strings"
)

// ===================== UTILIDAD =====================

func trimC(b []byte) string {
	return strings.TrimRight(string(b), "\x00 ")
}

// ensureImagePath garantiza que haya extensión válida y devuelve (rutaFinal, formatoGraphviz)
func ensureImagePath(path string) (string, string) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return path, "png"
	case ".jpg", ".jpeg":
		return path, "jpg"
	default:
		// sin extensión → forzar png
		return path + ".png", "png"
	}
}
