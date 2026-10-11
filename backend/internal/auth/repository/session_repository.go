package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// SessionRepository implementa domain.SessionRepository sobre las tablas sessions y auth_events.
// Firmas mínimas para las pruebas de T042: Create la implementa T046, FindByID la implementa US3 y
// Revoke la implementa US5.
type SessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository crea el repositorio de sesiones sobre la conexión recibida.
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create persiste la sesión y su evento login_succeeded en una sola transacción. Firma mínima: la
// implementa T046.
func (r *SessionRepository) Create(ctx context.Context, session *domain.Session, event *domain.AuthEvent) error {
	return errors.New("not implemented")
}

// FindByID busca la sesión por su jti. Firma mínima: la implementa US3.
func (r *SessionRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
	return nil, errors.New("not implemented")
}

// Revoke cierra la sesión y registra su evento logout. Firma mínima: la implementa US5.
func (r *SessionRepository) Revoke(ctx context.Context, id uuid.UUID, at time.Time, event *domain.AuthEvent) error {
	return errors.New("not implemented")
}
