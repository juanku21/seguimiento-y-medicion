package service

import (
	"context"
	"errors"
	"time"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// AuthService reúne los casos de uso de la autenticación sobre las interfaces de repositorio del
// dominio y el secreto de firma del JWT. Firma mínima para las pruebas de T041.
type AuthService struct {
	users     domain.UserRepository
	sessions  domain.SessionRepository
	events    domain.AuthEventRepository
	jwtSecret string
}

// NewAuthService compone el servicio con sus repositorios y el secreto de JWT_SECRET.
func NewAuthService(users domain.UserRepository, sessions domain.SessionRepository, events domain.AuthEventRepository, jwtSecret string) *AuthService {
	return &AuthService{users: users, sessions: sessions, events: events, jwtSecret: jwtSecret}
}

// LoginResult es lo que devuelve un inicio de sesión exitoso: el JWT y el vencimiento de la sesión
// (LoginResponse de contracts/openapi.yaml).
type LoginResult struct {
	Token     string
	ExpiresAt time.Time
}

// Login valida las credenciales y emite una sesión; ante cualquier credencial inválida devuelve
// domain.ErrInvalidCredentials (FR-014, RN-06). Firma mínima para las pruebas de T041; la
// implementación corresponde a T048.
func (s *AuthService) Login(ctx context.Context, email string, password string) (LoginResult, error) {
	return LoginResult{}, errors.New("not implemented")
}
