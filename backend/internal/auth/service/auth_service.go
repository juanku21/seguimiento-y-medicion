// Package service contiene la lógica de negocio del módulo de autenticación.
package service

import (
	"context"
	"errors"
	"fmt"
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
// domain.ErrInvalidCredentials (FR-014, RN-06). Recibe el correo ya normalizado por delivery, igual
// que Register (research.md sección 3). El correo inexistente y la contraseña incorrecta devuelven
// el mismo error y registran login_failed; los fallos de repositorio o de un hash corrupto se
// propagan envueltos, sin convertirlos en credenciales inválidas.
func (s *AuthService) Login(ctx context.Context, email string, password string) (LoginResult, error) {
	// Busca la cuenta y compara la contraseña; cada rechazo de credenciales registra el intento.
	user, err := s.users.FindByEmail(ctx, email)
	if errors.Is(err, domain.ErrInvalidCredentials) {
		return LoginResult{}, s.recordLoginFailed(ctx, nil, email)
	}
	if err != nil {
		return LoginResult{}, fmt.Errorf("buscar cuenta: %w", err)
	}
	err = ComparePassword(user.PasswordHash, password)
	if errors.Is(err, domain.ErrInvalidCredentials) {
		return LoginResult{}, s.recordLoginFailed(ctx, &user.ID, email)
	}
	if err != nil {
		return LoginResult{}, err
	}

	// Fija las fechas de la sesión desde un único instante, de modo que expires_at quede exactamente
	// domain.SessionLifetime después de created_at; la persiste con su evento login_succeeded y
	// emite el JWT con esas mismas fechas, para que iat y exp coincidan con las de la fila.
	now := time.Now()
	session := &domain.Session{
		ID:        uuid.New(),
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(domain.SessionLifetime),
	}
	event := &domain.AuthEvent{
		ID:        uuid.New(),
		EventType: domain.EventLoginSucceeded,
		UserID:    &user.ID,
	}
	if err := s.sessions.Create(ctx, session, event); err != nil {
		return LoginResult{}, fmt.Errorf("crear sesión: %w", err)
	}
	token, err := IssueToken(s.jwtSecret, *session)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, ExpiresAt: session.ExpiresAt}, nil
}

// recordLoginFailed registra el evento login_failed con el correo intentado y la cuenta, si existe
// (data-model.md sección 3), y devuelve el error genérico de credenciales inválidas. Si el evento no
// se puede persistir, propaga ese fallo envuelto.
func (s *AuthService) recordLoginFailed(ctx context.Context, userID *uuid.UUID, email string) error {
	event := &domain.AuthEvent{
		ID:             uuid.New(),
		EventType:      domain.EventLoginFailed,
		UserID:         userID,
		AttemptedEmail: &email,
	}
	if err := s.events.Create(ctx, event); err != nil {
		return fmt.Errorf("registrar inicio fallido: %w", err)
	}
	return domain.ErrInvalidCredentials
}
