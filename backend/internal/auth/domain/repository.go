package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// UserRepository declara solo las operaciones sobre users que usa esta feature (data-model.md
// sección 5; Principio III). Create persiste la cuenta y su evento account_created en una sola
// transacción; FindByEmail resuelve el inicio de sesión y FindByID la consulta del perfil. Si no
// encuentran el registro, devuelven el error de negocio y nunca gorm.ErrRecordNotFound ni
// (nil, nil): FindByEmail devuelve ErrInvalidCredentials (FR-014) y FindByID devuelve
// ErrInvalidSession, porque solo la usa GET /api/v1/users/me, cuyo único rechazo es la sesión
// inválida (FR-015).
type UserRepository interface {
	Create(ctx context.Context, user *User, event *AuthEvent) error
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
}

// SessionRepository declara solo las operaciones sobre sessions que usa esta feature. Create y
// Revoke reciben el evento para resolver la escritura de la sesión y la del evento en una sola
// transacción, sin exponer la transacción al servicio; FindByID verifica el jti en cada petición
// protegida. Si el id no existe, FindByID y Revoke devuelven ErrInvalidSession, nunca
// gorm.ErrRecordNotFound ni (nil, nil) (FR-015).
type SessionRepository interface {
	Create(ctx context.Context, session *Session, event *AuthEvent) error
	FindByID(ctx context.Context, id uuid.UUID) (*Session, error)
	Revoke(ctx context.Context, id uuid.UUID, at time.Time, event *AuthEvent) error
}

// AuthEventRepository persiste un evento aislado; esta feature lo usa solo para registrar el
// inicio de sesión fallido (login_failed).
type AuthEventRepository interface {
	Create(ctx context.Context, event *AuthEvent) error
}
