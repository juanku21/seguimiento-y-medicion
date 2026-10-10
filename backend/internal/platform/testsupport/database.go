// Package testsupport reúne los ayudantes de las pruebas de integración. Es un paquete normal, y no
// un archivo *_test.go, para que lo puedan importar las pruebas de cualquier módulo; el código de
// producción no lo importa (research.md sección 8).
package testsupport

import (
	"path/filepath"
	"runtime"
	"testing"

	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/config"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/database"
)

// testDatabaseName es la única base sobre la que el ayudante acepta operar (Principio V).
const testDatabaseName = "smye_test"

// OpenDatabase prepara la base de test para un caso de integración: carga backend/.env.test, abre
// verifica que la base configurada sea smye_test, abre la conexión migrando el esquema y vacía las
// tres tablas de la feature. Devuelve la conexión y la configuración cargada, y cierra el pool al terminar la prueba.
// Ante cualquier fallo detiene la prueba con t.Fatalf sin tocar ninguna base.
func OpenDatabase(t testing.TB) (*gorm.DB, config.Config) {
	t.Helper()

	// Se carga el .env.test del backend y se rechaza cualquier configuración que no apunte a la
	// base de test antes de conectarse, porque abrir la conexión ya migra el esquema.
	if err := config.LoadFile(envTestPath(t)); err != nil {
		t.Fatalf("no se pudo cargar el entorno de test: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("configuración de test inválida: %v", err)
	}
	if cfg.DBName != testDatabaseName {
		t.Fatalf("DB_NAME es %q: las pruebas de integración solo corren contra %q", cfg.DBName, testDatabaseName)
	}

	// Se abre la conexión y se registra el cierre del pool al finalizar la prueba.
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("no se pudo abrir la base de test: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("no se pudo obtener el pool de conexiones de test: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("no se pudo cerrar la conexión de test: %v", err)
		}
	})

	// Se vacían las tres tablas para que cada caso empiece sin datos de los anteriores.
	if err := db.Exec("TRUNCATE users, sessions, auth_events CASCADE").Error; err != nil {
		t.Fatalf("no se pudieron vaciar las tablas de test: %v", err)
	}
	return db, cfg
}

// envTestPath ubica backend/.env.test a partir de la ruta de este archivo fuente, de modo que se
// encuentra igual sin importar desde qué paquete corre la prueba.
func envTestPath(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("no se pudo determinar la ubicación del ayudante de pruebas")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", ".env.test")
}
