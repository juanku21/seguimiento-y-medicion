// Package testsupport reúne los ayudantes de las pruebas de integración. Es un paquete normal, y no
// un archivo *_test.go, para que lo puedan importar las pruebas de cualquier módulo; el código de
// producción no lo importa (research.md sección 8).
package testsupport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/config"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/database"
)

// testDatabaseName es la única base sobre la que el ayudante acepta operar (Principio V).
const testDatabaseName = "smye_test"

// testLockKey es la clave del advisory lock que serializa a los procesos de prueba que usan
// smye_test, y testLockTimeout es lo máximo que un proceso espera a que otro lo libere.
const (
	testLockKey     int64 = 2026_002_086
	testLockTimeout       = 5 * time.Minute
)

// lockOnce garantiza que cada proceso de prueba tome el lock una sola vez; lockConn conserva la
// conexión dedicada que lo sostiene hasta que el proceso termina, y lockErr guarda el resultado
// para que todas las llamadas posteriores lo informen.
var (
	lockOnce sync.Once
	lockConn *sql.Conn
	lockErr  error
)

// OpenDatabase prepara la base de test para un caso de integración: carga backend/.env.test,
// verifica que la base configurada sea smye_test, en la primera llamada del proceso toma el lock
// que serializa el acceso de los paquetes de prueba a esa base, abre la conexión migrando el
// esquema y vacía las tablas de autenticación y de proyectos. Devuelve la conexión y la
// configuración cargada, y cierra el pool al terminar la prueba. Ante cualquier fallo detiene la
// prueba con t.Fatalf sin tocar ninguna base.
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

	// Antes de migrar y de vaciar tablas, el proceso toma el lock exclusivo de smye_test para que
	// otro paquete de pruebas corriendo en paralelo no le borre los datos.
	lockOnce.Do(func() { lockErr = acquireTestLock(cfg) })
	if lockErr != nil {
		t.Fatalf("no se pudo tomar el lock de la base de test: %v", lockErr)
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

	// Se vacían las tablas de autenticación y de proyectos para que cada caso empiece sin datos de
	// los anteriores.
	if err := db.Exec("TRUNCATE users, sessions, auth_events, projects, project_members CASCADE").Error; err != nil {
		t.Fatalf("no se pudieron vaciar las tablas de test: %v", err)
	}
	return db, cfg
}

// acquireTestLock abre una conexión dedicada, fuera del pool de GORM, y toma en ella
// pg_advisory_lock con la clave de test, esperando como máximo testLockTimeout. El lock no se
// libera: PostgreSQL lo suelta al cerrarse la conexión cuando termina el proceso de prueba. Si algo
// falla, cierra lo que haya abierto y devuelve el error.
func acquireTestLock(cfg config.Config) error {
	pool, err := sql.Open("pgx", database.DSN(cfg))
	if err != nil {
		return fmt.Errorf("no se pudo preparar la conexión del lock: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), testLockTimeout)
	defer cancel()
	conn, err := pool.Conn(ctx)
	if err != nil {
		return fmt.Errorf("no se pudo abrir la conexión del lock: %w", errors.Join(err, pool.Close()))
	}
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", testLockKey); err != nil {
		lockFailure := fmt.Errorf("pg_advisory_lock falló o superó la espera máxima de %s: %w", testLockTimeout, err)
		return errors.Join(lockFailure, conn.Close(), pool.Close())
	}
	lockConn = conn
	return nil
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
