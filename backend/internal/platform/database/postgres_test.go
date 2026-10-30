package database

import (
	"testing"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/config"
)

// Con DATABASE_URL configurada, DSN la devuelve sin cambios, para respetar el host, la base y el
// sslmode que indique el proveedor.
func TestDSN_UsesDatabaseURL(t *testing.T) {
	const databaseURL = "postgres://user:password@dpg-example-a:5432/smye?sslmode=require"
	cfg := config.Config{
		DatabaseURL: databaseURL,
		DBHost:      "localhost",
		DBPort:      "5432",
		DBUser:      "other",
		DBPassword:  "other",
		DBName:      "other",
	}

	if got := DSN(cfg); got != databaseURL {
		t.Errorf("DSN() = %q; se esperaba %q", got, databaseURL)
	}
}

// Sin DATABASE_URL, DSN arma la URL con las variables DB_*, escapa la contraseña y desactiva el
// SSL del contenedor local.
func TestDSN_BuildsFromDBVars(t *testing.T) {
	cfg := config.Config{
		DBHost:     "localhost",
		DBPort:     "5433",
		DBUser:     "smye",
		DBPassword: "p@ss/word",
		DBName:     "smye_test",
	}

	const want = "postgres://smye:p%40ss%2Fword@localhost:5433/smye_test?sslmode=disable"
	if got := DSN(cfg); got != want {
		t.Errorf("DSN() = %q; se esperaba %q", got, want)
	}
}
