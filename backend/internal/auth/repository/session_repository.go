package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// sessionLifetime es el plazo fijo de una sesión desde su emisión; no se renueva con el uso
// (FR-013, FR-018).
const sessionLifetime = 8 * time.Hour

// SessionRepository implementa domain.SessionRepository sobre las tablas sessions y auth_events.
// Create está implementado; FindByID y Revoke son firmas mínimas que corresponden a las historias
// US3 y US5.
type SessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository crea el repositorio de sesiones sobre la conexión recibida.
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create persiste la sesión y su evento login_succeeded en una sola transacción, para que no quede
// sesión sin evento ni evento sin sesión (research.md sección 5; FR-028). El momento de emisión es
// created_at (research.md sección 6): se fija acá y expires_at se deriva de ese mismo instante, de
// modo que la diferencia sea exactamente sessionLifetime (FR-013, FR-018). GORM respeta el
// CreatedAt ya asignado. Cualquier error revierte la transacción completa.
func (r *SessionRepository) Create(ctx context.Context, session *domain.Session, event *domain.AuthEvent) error {
	session.CreatedAt = time.Now()
	session.ExpiresAt = session.CreatedAt.Add(sessionLifetime)

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(session).Error; err != nil {
			return fmt.Errorf("insertar sesión: %w", err)
		}
		if err := tx.Create(event).Error; err != nil {
			return fmt.Errorf("insertar evento %s: %w", event.EventType, err)
		}
		return nil
	})
}

// FindByID busca la sesión por su jti. Firma mínima: la implementa US3.
func (r *SessionRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
	return nil, errors.New("not implemented")
}

// Revoke cierra la sesión y registra su evento logout. Firma mínima: la implementa US5.
func (r *SessionRepository) Revoke(ctx context.Context, id uuid.UUID, at time.Time, event *domain.AuthEvent) error {
	return errors.New("not implemented")
}
