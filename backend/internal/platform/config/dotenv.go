package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadFile lee un archivo de líneas KEY=VALUE y carga cada par en el entorno del proceso, para que
// las pruebas encuentren la configuración de backend/.env.test sin exportar variables a mano
// (research.md sección 8). Ignora las líneas vacías y los comentarios que empiezan con '#'. Los
// valores del archivo reemplazan a los ya presentes en el entorno, de modo que el entorno de test
// queda definido exclusivamente por su .env.test (Principio V).
func LoadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("no se pudo abrir el archivo de entorno %s: %w", path, err)
	}
	defer file.Close()

	// Se recorre el archivo línea por línea y se rechaza cualquier línea sin '=' o sin clave,
	// indicando su número para que el error sea fácil de corregir.
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			return fmt.Errorf("línea %d de %s inválida: se esperaba KEY=VALUE", lineNumber, path)
		}
		if err := os.Setenv(key, strings.TrimSpace(value)); err != nil {
			return fmt.Errorf("no se pudo cargar %s desde %s: %w", key, path, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("no se pudo leer el archivo de entorno %s: %w", path, err)
	}
	return nil
}
