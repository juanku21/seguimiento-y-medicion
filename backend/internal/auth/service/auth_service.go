// Package service contiene la lógica de negocio del módulo de autenticación.
package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// AuthService reúne los casos de uso de la autenticación sobre las interfaces de repositorio del
// dominio y el secreto de firma del JWT.
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
