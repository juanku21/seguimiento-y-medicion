package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// AuthEventRepository implementa domain.AuthEventRepository sobre la tabla auth_events, para los
// eventos que no acompañan a otro efecto persistido (login_failed).
type AuthEventRepository struct {
	db *gorm.DB
}

// NewAuthEventRepository crea el repositorio de eventos sobre la conexión recibida.
func NewAuthEventRepository(db *gorm.DB) *AuthEventRepository {
	return &AuthEventRepository{db: db}
}

// Create persiste un evento aislado, como login_failed, con una inserción simple: al ser una sola
// fila no necesita transacción (research.md sección 5). El evento llega completo desde el servicio
// (ID, user_id nulo si el correo no existe y attempted_email normalizado); GORM completa
// created_at y updated_at (FR-029).
func (r *AuthEventRepository) Create(ctx context.Context, event *domain.AuthEvent) error {
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("insertar evento %s: %w", event.EventType, err)
	}
	return nil
}
