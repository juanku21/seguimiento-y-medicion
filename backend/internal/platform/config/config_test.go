package config

import (
	"os"
	"strings"
	"testing"
)

// setRequiredEnv completa con t.Setenv todas las variables obligatorias que lee Load, con valores
// válidos y un JWT_SECRET de la longitud mínima, para que cada prueba solo varíe APP_TIMEZONE y no
// dependa del entorno de la máquina ni de los archivos .env.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "user")
	t.Setenv("DB_PASSWORD", "password")
	t.Setenv("DB_NAME", "db")
	t.Setenv("JWT_SECRET", strings.Repeat("s", MinJWTSecretBytes))
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("CORS_ALLOWED_ORIGIN", "http://localhost:3000")
}

// Sin APP_TIMEZONE, o con un valor de solo espacios, Load falla y la enumera entre las variables
// obligatorias faltantes (FR-039).
func TestLoad_AppTimezoneMissing(t *testing.T) {
	// Cada caso prepara APP_TIMEZONE: "ausente" la quita del entorno (t.Setenv previo garantiza que
	// se restaure al terminar) y "solo espacios" la define con blancos.
	cases := map[string]func(t *testing.T){
		"ausente": func(t *testing.T) {
			t.Setenv("APP_TIMEZONE", "")
			if err := os.Unsetenv("APP_TIMEZONE"); err != nil {
				t.Fatalf("no se pudo quitar APP_TIMEZONE: %v", err)
			}
		},
		"solo espacios": func(t *testing.T) { t.Setenv("APP_TIMEZONE", "   \t ") },
	}
	for name, prepare := range cases {
		t.Run(name, func(t *testing.T) {
			setRequiredEnv(t)
			prepare(t)

			cfg, err := Load()
			if err == nil {
				t.Fatalf("se esperaba error y Load devolvió %+v", cfg)
			}
			msg := err.Error()
			if !strings.Contains(msg, "faltan variables de entorno obligatorias") {
				t.Errorf("el error no informa variables faltantes: %q", msg)
			}
			if !strings.Contains(msg, "APP_TIMEZONE") {
				t.Errorf("el error no enumera APP_TIMEZONE: %q", msg)
			}
			if cfg.Location != nil {
				t.Errorf("con error, Location debía ser nil y es %v", cfg.Location)
			}
		})
	}
}

// Con un valor que no es una zona IANA, Load devuelve el error de zona horaria inválida, que nombra
// la variable y el valor recibido, y no la informa como faltante (FR-039).
func TestLoad_AppTimezoneInvalid(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_TIMEZONE", "Buenos_Aires")

	_, err := Load()
	if err == nil {
		t.Fatal("se esperaba error por zona horaria inválida y Load no devolvió error")
	}
	msg := err.Error()
	if !strings.Contains(msg, `APP_TIMEZONE "Buenos_Aires"`) {
		t.Errorf("el error no nombra la variable y el valor: %q", msg)
	}
	if !strings.Contains(msg, "no es una zona horaria válida") {
		t.Errorf("el error no indica zona horaria inválida: %q", msg)
	}
	if strings.Contains(msg, "faltan variables de entorno obligatorias") {
		t.Errorf("el error no debía informar variables faltantes: %q", msg)
	}
}

// Con una zona IANA válida, Load no falla y Config.Location corresponde al valor configurado
// (FR-039).
func TestLoad_AppTimezoneValid(t *testing.T) {
	const timezone = "America/Argentina/Buenos_Aires"
	setRequiredEnv(t)
	t.Setenv("APP_TIMEZONE", timezone)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load devolvió un error inesperado: %v", err)
	}
	if cfg.Location == nil {
		t.Fatal("Config.Location es nil")
	}
	if got := cfg.Location.String(); got != timezone {
		t.Errorf("Config.Location.String() = %q; se esperaba %q", got, timezone)
	}
}
