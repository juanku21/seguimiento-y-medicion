// Package service contiene la lógica de negocio del módulo de autenticación.
package service

import (
	"context"
	"errors"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// AuthService resuelve el registro de cuentas sobre el repositorio de usuarios. Firma mínima creada
// por T027 para que compilen las pruebas de integración; la implementación corresponde a T032.
type AuthService struct {
	users domain.UserRepository
}

// NewAuthService crea el servicio de autenticación con el repositorio de usuarios recibido.
func NewAuthService(users domain.UserRepository) *AuthService {
	return &AuthService{users: users}
}

// Register crea la cuenta con la contraseña hasheada y su evento account_created, sin emitir sesión
// (FR-009). Recibe el nombre y el correo ya validados y normalizados por delivery. Firma mínima: la
// implementa T032.
func (s *AuthService) Register(ctx context.Context, fullName, email, password string) (*domain.User, error) {
	return nil, errors.New("not implemented")
}
