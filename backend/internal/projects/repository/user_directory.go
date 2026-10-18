package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// UserDirectory implementa domain.UserDirectory con GORM: es el único punto del módulo de proyectos
// que lee la tabla users (research.md sección 8).
type UserDirectory struct {
	db *gorm.DB
}

// NewUserDirectory crea el directorio de usuarios sobre la conexión recibida.
func NewUserDirectory(db *gorm.DB) *UserDirectory {
	return &UserDirectory{db: db}
}

// FindByEmail busca la cuenta con el correo ya normalizado por el servicio. La consulta enumera
// solo users.id, users.full_name y users.email, de modo que password_hash nunca se selecciona ni
// entra al módulo (FR-019, FR-020). Si no hay cuenta con ese correo devuelve ErrUserNotRegistered
// (FR-021), nunca gorm.ErrRecordNotFound ni (nil, nil).
func (d *UserDirectory) FindByEmail(ctx context.Context, normalizedEmail string) (*domain.UserRef, error) {
	var ref domain.UserRef
	err := d.db.WithContext(ctx).
		Table("users").
		Select("users.id, users.full_name, users.email").
		Where("users.email = ?", normalizedEmail).
		Take(&ref).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotRegistered
	}
	if err != nil {
		return nil, fmt.Errorf("buscar la cuenta por correo: %w", err)
	}
	return &ref, nil
}
