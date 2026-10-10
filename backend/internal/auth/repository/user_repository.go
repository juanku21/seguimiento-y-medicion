// Package repository persiste las entidades del módulo de autenticación con GORM.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// UserRepository implementa domain.UserRepository sobre la tabla users. Firma mínima creada por
// T027 para que compilen las pruebas de integración; Create lo implementa T031 y FindByEmail y
// FindByID corresponden a las historias US2 y US4.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository crea el repositorio de usuarios sobre la conexión recibida.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create persiste la cuenta y su evento account_created en una sola transacción, para que no quede
// cuenta sin evento ni evento sin cuenta (research.md sección 5; FR-008, FR-028). La unicidad del
// correo la garantiza el índice único de users: se inserta directamente, sin consultar antes, y la
// violación del índice (gorm.ErrDuplicatedKey, gracias a TranslateError) se traduce a
// domain.ErrEmailUnavailable, lo que cubre también los registros concurrentes (FR-007, SC-006).
// Cualquier error revierte la transacción completa.
func (r *UserRepository) Create(ctx context.Context, user *domain.User, event *domain.AuthEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return domain.ErrEmailUnavailable
			}
			return fmt.Errorf("insertar usuario: %w", err)
		}
		if err := tx.Create(event).Error; err != nil {
			return fmt.Errorf("insertar evento %s: %w", event.EventType, err)
		}
		return nil
	})
}

// FindByEmail busca la cuenta por correo normalizado. Firma mínima: la implementa US2.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, errors.New("not implemented")
}

// FindByID busca la cuenta por identificador. Firma mínima: la implementa US4.
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return nil, errors.New("not implemented")
}
