package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// UserDirectory implementa domain.UserDirectory con GORM: es el único punto del módulo de proyectos
// que lee la tabla users (research.md sección 8). Firma mínima de T044: FindByEmail la implementa
// T047.
type UserDirectory struct {
	db *gorm.DB
}

// NewUserDirectory crea el directorio de usuarios sobre la conexión recibida.
func NewUserDirectory(db *gorm.DB) *UserDirectory {
	return &UserDirectory{db: db}
}

// FindByEmail busca la cuenta con el correo ya normalizado. Firma mínima de T044.
func (d *UserDirectory) FindByEmail(ctx context.Context, normalizedEmail string) (*domain.UserRef, error) {
	return nil, errors.New("not implemented")
}
