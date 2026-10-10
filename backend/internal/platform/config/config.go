// Package config lee la configuración de la aplicación desde variables de entorno usando solo la
// biblioteca estándar (Principio IV) y valida al arrancar los valores que no admiten defaults.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// MinJWTSecretBytes es la longitud mínima del secreto de firma HS256 (research.md sección 2).
const MinJWTSecretBytes = 32

// Config agrupa las variables de entorno que necesita el backend; sus claves coinciden con
// backend/.env.example.
type Config struct {
	DBHost            string
	DBPort            string
	DBUser            string
	DBPassword        string
	DBName            string
	JWTSecret         string
	ServerPort        string
	CORSAllowedOrigin string
}

// Load lee cada variable del entorno del proceso y devuelve un error que enumera todas las que
// faltan o están vacías, más el aviso de un JWT_SECRET demasiado corto, para que la aplicación no
// arranque con una configuración incompleta.
func Load() (Config, error) {
	var missing []string
	read := func(key string) string {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			missing = append(missing, key)
		}
		return value
	}

	cfg := Config{
		DBHost:            read("DB_HOST"),
		DBPort:            read("DB_PORT"),
		DBUser:            read("DB_USER"),
		DBPassword:        read("DB_PASSWORD"),
		DBName:            read("DB_NAME"),
		JWTSecret:         read("JWT_SECRET"),
		ServerPort:        read("SERVER_PORT"),
		CORSAllowedOrigin: read("CORS_ALLOWED_ORIGIN"),
	}

	// Se acumulan los problemas para informarlos todos juntos en un solo arranque fallido.
	var errs []error
	if len(missing) > 0 {
		errs = append(errs, fmt.Errorf("faltan variables de entorno obligatorias: %s", strings.Join(missing, ", ")))
	}
	if cfg.JWTSecret != "" && len(cfg.JWTSecret) < MinJWTSecretBytes {
		errs = append(errs, fmt.Errorf("JWT_SECRET debe tener al menos %d bytes (tiene %d)", MinJWTSecretBytes, len(cfg.JWTSecret)))
	}
	if len(errs) > 0 {
		return Config{}, fmt.Errorf("configuración inválida: %w", errors.Join(errs...))
	}
	return cfg, nil
}
