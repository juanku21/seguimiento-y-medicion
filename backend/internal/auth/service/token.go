package service

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// TokenClaims son los datos que el JWT transporta de la sesión: jti (SessionID), sub (UserID),
// iat (IssuedAt) y exp (ExpiresAt), según data-model.md sección 2.
type TokenClaims struct {
	SessionID uuid.UUID
	UserID    uuid.UUID
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// IssueToken firma con HS256 y el secreto de JWT_SECRET un JWT cuyos claims reflejan la sesión.
// Firma mínima para las pruebas de T040; la implementación corresponde a T044.
func IssueToken(secret string, session domain.Session) (string, error) {
	return "", errors.New("not implemented")
}

// ParseToken valida la firma HS256 y el vencimiento del token respecto de now y devuelve sus
// claims. Firma mínima para las pruebas de T040; la implementación corresponde a T044.
func ParseToken(secret string, token string, now time.Time) (TokenClaims, error) {
	return TokenClaims{}, errors.New("not implemented")
}
