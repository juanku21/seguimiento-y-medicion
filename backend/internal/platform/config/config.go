// Package config lee la configuración de la aplicación desde variables de entorno usando solo la
// biblioteca estándar (Principio IV) y valida al arrancar los valores que no admiten defaults.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// MinJWTSecretBytes es la longitud mínima del secreto de firma HS256 (research.md sección 2).
const MinJWTSecretBytes = 32

// defaultEffortGracePeriodHours es el plazo de gracia, en horas, que se usa cuando
// EFFORT_GRACE_PERIOD_HOURS falta o está vacía (RC-11).
const defaultEffortGracePeriodHours = 48

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
	// Location es la zona horaria única del sistema, cargada desde APP_TIMEZONE (research.md
	// sección 4).
	Location *time.Location
	// EffortGracePeriod es el plazo de gracia para registrar y corregir esfuerzo después de cerrar
	// un sprint, cargado desde EFFORT_GRACE_PERIOD_HOURS (RC-11).
	EffortGracePeriod time.Duration
}

// Load lee cada variable del entorno del proceso y devuelve un error que enumera todas las que
// faltan o están vacías, más el aviso de un JWT_SECRET demasiado corto, de un APP_TIMEZONE que no
// es una zona horaria válida o de un EFFORT_GRACE_PERIOD_HOURS que no es un entero mayor o igual
// que 1, para que la aplicación no arranque con una configuración incompleta.
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
	timezone := read("APP_TIMEZONE")
	graceHours := strings.TrimSpace(os.Getenv("EFFORT_GRACE_PERIOD_HOURS"))

	// Se acumulan los problemas para informarlos todos juntos en un solo arranque fallido.
	var errs []error
	if len(missing) > 0 {
		errs = append(errs, fmt.Errorf("faltan variables de entorno obligatorias: %s", strings.Join(missing, ", ")))
	}
	if cfg.JWTSecret != "" && len(cfg.JWTSecret) < MinJWTSecretBytes {
		errs = append(errs, fmt.Errorf("JWT_SECRET debe tener al menos %d bytes (tiene %d)", MinJWTSecretBytes, len(cfg.JWTSecret)))
	}
	if timezone != "" {
		location, err := time.LoadLocation(timezone)
		if err != nil {
			errs = append(errs, fmt.Errorf("APP_TIMEZONE %q no es una zona horaria válida: %w", timezone, err))
		}
		cfg.Location = location
	}
	// El plazo de gracia es opcional: sin valor se usa el defecto y, con valor, debe ser un entero
	// de horas mayor o igual que 1.
	cfg.EffortGracePeriod = defaultEffortGracePeriodHours * time.Hour
	if graceHours != "" {
		hours, err := strconv.Atoi(graceHours)
		if err != nil || hours < 1 {
			errs = append(errs, fmt.Errorf("EFFORT_GRACE_PERIOD_HOURS %q debe ser un entero de horas mayor o igual que 1", graceHours))
		}
		cfg.EffortGracePeriod = time.Duration(hours) * time.Hour
	}
	if len(errs) > 0 {
		return Config{}, fmt.Errorf("configuración inválida: %w", errors.Join(errs...))
	}
	return cfg, nil
}
