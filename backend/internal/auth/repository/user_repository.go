// Package repository persiste las entidades del módulo de autenticación con GORM.
package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// UserRepository implementa domain.UserRepository sobre la tabla users. Firma mínima creada por
// T027 para que compilen las pruebas de integración; la implementación corresponde a T031 (Create)
// y a las historias US2 (FindByEmail) y US4 (FindByID).
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository crea el repositorio de usuarios sobre la conexión recibida.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create persiste la cuenta y su evento account_created en una sola transacción. Firma mínima: la
// implementa T031.
func (r *UserRepository) Create(ctx context.Context, user *domain.User, event *domain.AuthEvent) error {
	return errors.New("not implemented")
}

// FindByEmail busca la cuenta por correo normalizado. Firma mínima: la implementa US2.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, errors.New("not implemented")
}

// FindByID busca la cuenta por identificador. Firma mínima: la implementa US4.
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return nil, errors.New("not implemented")
}
