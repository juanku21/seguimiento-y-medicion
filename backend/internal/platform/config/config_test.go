package config

import (
	"os"
	"strings"
	"testing"
	"time"
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

// setGraceEnv prepara EFFORT_GRACE_PERIOD_HOURS para una prueba: con present en false la quita del
// entorno (el t.Setenv previo garantiza que se restaure al terminar) y si no la define con value.
func setGraceEnv(t *testing.T, value string, present bool) {
	t.Helper()
	t.Setenv("EFFORT_GRACE_PERIOD_HOURS", value)
	if !present {
		if err := os.Unsetenv("EFFORT_GRACE_PERIOD_HOURS"); err != nil {
			t.Fatalf("no se pudo quitar EFFORT_GRACE_PERIOD_HOURS: %v", err)
		}
	}
}

// Sin EFFORT_GRACE_PERIOD_HOURS, o con la variable vacía, Load no falla y el plazo de gracia es de
// 48 horas, porque la variable es opcional (006/US1, RC-11).
func TestLoad_EffortGracePeriodDefault(t *testing.T) {
	cases := map[string]bool{
		"ausente": false,
		"vacía":   true,
	}
	for name, present := range cases {
		t.Run(name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("APP_TIMEZONE", "America/Argentina/Buenos_Aires")
			setGraceEnv(t, "", present)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load devolvió un error inesperado: %v", err)
			}
			if cfg.EffortGracePeriod != 48*time.Hour {
				t.Errorf("Config.EffortGracePeriod = %v; se esperaba %v", cfg.EffortGracePeriod, 48*time.Hour)
			}
		})
	}
}

// Con un entero de horas mayor o igual que 1, Load no falla y el plazo de gracia es ese número de
// horas, incluido el mínimo de 1 (006/US1, RC-11).
func TestLoad_EffortGracePeriodValid(t *testing.T) {
	cases := map[string]time.Duration{
		"48": 48 * time.Hour,
		"1":  time.Hour,
	}
	for value, want := range cases {
		t.Run(value, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("APP_TIMEZONE", "America/Argentina/Buenos_Aires")
			setGraceEnv(t, value, true)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load devolvió un error inesperado: %v", err)
			}
			if cfg.EffortGracePeriod != want {
				t.Errorf("Config.EffortGracePeriod = %v; se esperaba %v", cfg.EffortGracePeriod, want)
			}
		})
	}
}

// Con un valor que no es un entero mayor o igual que 1, Load falla con un error que nombra la
// variable, para que la aplicación no arranque, y no la informa como faltante (006/US1, RC-11).
func TestLoad_EffortGracePeriodInvalid(t *testing.T) {
	for _, value := range []string{"0", "-1", "48h", "1.5", "abc"} {
		t.Run(value, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("APP_TIMEZONE", "America/Argentina/Buenos_Aires")
			setGraceEnv(t, value, true)

			cfg, err := Load()
			if err == nil {
				t.Fatalf("se esperaba error por plazo de gracia inválido y Load devolvió %+v", cfg)
			}
			msg := err.Error()
			if !strings.Contains(msg, "EFFORT_GRACE_PERIOD_HOURS") {
				t.Errorf("el error no nombra EFFORT_GRACE_PERIOD_HOURS: %q", msg)
			}
			if strings.Contains(msg, "faltan variables de entorno obligatorias") {
				t.Errorf("el error no debía informar variables faltantes: %q", msg)
			}
			if cfg.EffortGracePeriod != 0 {
				t.Errorf("con error, EffortGracePeriod debía ser 0 y es %v", cfg.EffortGracePeriod)
			}
		})
	}
}

// Un plazo de gracia inválido se acumula con los demás problemas de configuración: junto con un
// APP_TIMEZONE inválido, el único error de Load nombra ambas variables (006/US1, RC-11).
func TestLoad_EffortGracePeriodInvalidAccumulated(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_TIMEZONE", "Buenos_Aires")
	setGraceEnv(t, "abc", true)

	_, err := Load()
	if err == nil {
		t.Fatal("se esperaba error por configuración inválida y Load no devolvió error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "EFFORT_GRACE_PERIOD_HOURS") {
		t.Errorf("el error no nombra EFFORT_GRACE_PERIOD_HOURS: %q", msg)
	}
	if !strings.Contains(msg, `APP_TIMEZONE "Buenos_Aires"`) {
		t.Errorf("el error no conserva el problema de APP_TIMEZONE: %q", msg)
	}
}

// unsetEnv quita del entorno cada clave indicada; el t.Setenv previo garantiza que se restaure el
// valor original al terminar la prueba.
func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("no se pudo quitar %s: %v", key, err)
		}
	}
}

// dbVars son las variables de conexión que DATABASE_URL reemplaza cuando está definida.
var dbVars = []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME"}

// Con DATABASE_URL definida, Load no exige las variables DB_* y conserva la URL tal cual, para
// conectarse con la Internal Database URL de Render.
func TestLoad_DatabaseURLReplacesDBVars(t *testing.T) {
	const databaseURL = "postgres://user:password@dpg-example-a:5432/smye?sslmode=require"
	setRequiredEnv(t)
	t.Setenv("APP_TIMEZONE", "America/Argentina/Buenos_Aires")
	unsetEnv(t, dbVars...)
	t.Setenv("DATABASE_URL", "  "+databaseURL+"  ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load devolvió un error inesperado: %v", err)
	}
	if cfg.DatabaseURL != databaseURL {
		t.Errorf("Config.DatabaseURL = %q; se esperaba %q", cfg.DatabaseURL, databaseURL)
	}
}

// Sin DATABASE_URL, o con un valor de solo espacios, las variables DB_* siguen siendo
// obligatorias y Load las enumera como faltantes.
func TestLoad_DatabaseURLMissingRequiresDBVars(t *testing.T) {
	cases := map[string]func(t *testing.T){
		"ausente":       func(t *testing.T) { unsetEnv(t, "DATABASE_URL") },
		"solo espacios": func(t *testing.T) { t.Setenv("DATABASE_URL", "   ") },
	}
	for name, prepare := range cases {
		t.Run(name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("APP_TIMEZONE", "America/Argentina/Buenos_Aires")
			unsetEnv(t, dbVars...)
			prepare(t)

			_, err := Load()
			if err == nil {
				t.Fatal("se esperaba error por variables DB_* faltantes y Load no devolvió error")
			}
			msg := err.Error()
			for _, key := range dbVars {
				if !strings.Contains(msg, key) {
					t.Errorf("el error no enumera %s: %q", key, msg)
				}
			}
		})
	}
}
