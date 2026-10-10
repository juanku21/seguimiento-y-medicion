// Package service contiene la lógica de negocio del módulo de autenticación.
package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// AuthService resuelve el registro de cuentas sobre el repositorio de usuarios.
type AuthService struct {
	users domain.UserRepository
}

// NewAuthService crea el servicio de autenticación con el repositorio de usuarios recibido.
func NewAuthService(users domain.UserRepository) *AuthService {
	return &AuthService{users: users}
}

// Register crea la cuenta con la contraseña hasheada y su evento account_created, sin emitir sesión
// (FR-009). Recibe el nombre y el correo ya validados y normalizados por delivery. Los
// identificadores los genera la aplicación y las fechas las completa GORM al insertar. Los errores
// se propagan tal cual, de modo que domain.ErrEmailUnavailable siga reconocible con errors.Is.
func (s *AuthService) Register(ctx context.Context, fullName, email, password string) (*domain.User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		ID:           uuid.New(),
		FullName:     fullName,
		Email:        email,
		PasswordHash: hash,
	}
	event := &domain.AuthEvent{
		ID:        uuid.New(),
		EventType: domain.EventAccountCreated,
		UserID:    &user.ID,
	}

	if err := s.users.Create(ctx, user, event); err != nil {
		return nil, err
	}
	return user, nil
}
