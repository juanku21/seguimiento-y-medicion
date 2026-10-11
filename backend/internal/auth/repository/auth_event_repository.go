package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// AuthEventRepository implementa domain.AuthEventRepository sobre la tabla auth_events. Firma
// mínima para las pruebas de T042: Create la implementa T047.
type AuthEventRepository struct {
	db *gorm.DB
}

// NewAuthEventRepository crea el repositorio de eventos sobre la conexión recibida.
func NewAuthEventRepository(db *gorm.DB) *AuthEventRepository {
	return &AuthEventRepository{db: db}
}

// Create persiste un evento aislado (login_failed). Firma mínima: la implementa T047.
func (r *AuthEventRepository) Create(ctx context.Context, event *domain.AuthEvent) error {
	return errors.New("not implemented")
}
