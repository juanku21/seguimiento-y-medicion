// Package database abre la conexión GORM con PostgreSQL y crea o actualiza el esquema con
// AutoMigrate, sin herramientas de migración externas (research.md sección 7).
package database

import (
	"errors"
	"fmt"
	"net"
	"net/url"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/config"
)

// Open se conecta a la base indicada por la configuración con TranslateError activado, para que
// las violaciones de restricciones lleguen como errores de GORM (por ejemplo gorm.ErrDuplicatedKey)
// y no como códigos propios de PostgreSQL (research.md sección 3), y migra los tres modelos de la
// feature de autenticación. Lo usan cmd/app al arrancar y el ayudante de las pruebas de integración.
func Open(cfg config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn(cfg)), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar con PostgreSQL en %s:%s: %w", cfg.DBHost, cfg.DBPort, err)
	}

	// User se migra primero porque sessions y auth_events declaran claves foráneas hacia users.id.
	// Si la migración falla se cierra la conexión para no dejar el pool abierto.
	if err := db.AutoMigrate(&domain.User{}, &domain.Session{}, &domain.AuthEvent{}); err != nil {
		migrateErr := fmt.Errorf("no se pudo migrar el esquema: %w", err)
		sqlDB, dbErr := db.DB()
		if dbErr != nil {
			return nil, errors.Join(migrateErr, dbErr)
		}
		return nil, errors.Join(migrateErr, sqlDB.Close())
	}
	return db, nil
}

// dsn arma la URL de conexión con net/url para que una contraseña con caracteres especiales quede
// escapada. El SSL se desactiva porque la base corre en un contenedor local de Docker Compose.
func dsn(cfg config.Config) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.DBUser, cfg.DBPassword),
		Host:     net.JoinHostPort(cfg.DBHost, cfg.DBPort),
		Path:     cfg.DBName,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}
