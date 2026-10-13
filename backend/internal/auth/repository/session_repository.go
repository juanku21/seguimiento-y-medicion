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

// SessionRepository implementa domain.SessionRepository sobre las tablas sessions y auth_events.
type SessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository crea el repositorio de sesiones sobre la conexión recibida.
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create persiste la sesión y su evento login_succeeded en una sola transacción, para que no quede
// sesión sin evento ni evento sin sesión (research.md sección 5; FR-028). La sesión se guarda tal
// como llega: CreatedAt y ExpiresAt los fija el servicio y GORM respeta el CreatedAt ya asignado.
// Cualquier error revierte la transacción completa.
func (r *SessionRepository) Create(ctx context.Context, session *domain.Session, event *domain.AuthEvent) error {
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

// FindByID busca la sesión por su jti para verificarla en cada petición protegida (FR-026). Solo
// resuelve la existencia de la fila: la vigencia (revoked_at y expires_at) la decide el servicio.
// Si no existe, devuelve domain.ErrInvalidSession y nunca (nil, nil) (FR-015); cualquier otro error
// se envuelve y se propaga.
func (r *SessionRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
	var session domain.Session
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrInvalidSession
		}
		return nil, fmt.Errorf("buscar sesión por id: %w", err)
	}
	return &session, nil
}

// Revoke cierra la sesión indicada y registra su evento logout en una sola transacción (research.md
// sección 5; FR-026). El UPDATE filtra por id, así que solo toca esa fila y las demás sesiones de
// la cuenta siguen vigentes (FR-017); además exige revoked_at nulo, para que una sesión ya cerrada
// no cambie su momento de cierre ni sume un segundo evento. Si no se actualizó ninguna fila (id
// inexistente o sesión ya cerrada) devuelve domain.ErrInvalidSession y revierte la transacción.
func (r *SessionRepository) Revoke(ctx context.Context, id uuid.UUID, at time.Time, event *domain.AuthEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&domain.Session{}).
			Where("id = ? AND revoked_at IS NULL", id).
			Update("revoked_at", at)
		if result.Error != nil {
			return fmt.Errorf("revocar sesión: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return domain.ErrInvalidSession
		}
		if err := tx.Create(event).Error; err != nil {
			return fmt.Errorf("insertar evento %s: %w", event.EventType, err)
		}
		return nil
	})
}
