package service

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

// IssueToken firma con HS256 y el secreto de JWT_SECRET un JWT cuyos claims reflejan la sesión:
// jti es sessions.id, sub es users.id, iat es sessions.created_at y exp es sessions.expires_at.
func IssueToken(secret string, session domain.Session) (string, error) {
	claims := jwt.RegisteredClaims{
		ID:        session.ID.String(),
		Subject:   session.UserID.String(),
		IssuedAt:  jwt.NewNumericDate(session.CreatedAt),
		ExpiresAt: jwt.NewNumericDate(session.ExpiresAt),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("firmar token: %w", err)
	}
	return token, nil
}

// ParseToken valida la firma HS256 y el vencimiento del token respecto de now y devuelve sus
// claims. Solo acepta HS256 y exige exp e iat; el token vence cuando exp <= now. Cualquier token
// rechazado (firma, algoritmo, formato, vencimiento o claims ilegibles) se informa como
// domain.ErrInvalidSession, para que el llamador lo trate como falta de sesión (FR-015).
func ParseToken(secret string, token string, now time.Time) (TokenClaims, error) {
	// Verifica firma, algoritmo y vencimiento con la librería, usando now como reloj.
	keyFunc := func(*jwt.Token) (any, error) { return []byte(secret), nil }
	claims := jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, &claims, keyFunc,
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		return TokenClaims{}, invalidSession("%v", err)
	}

	// Los claims jti y sub deben ser UUID válidos. iat se comprueba aparte porque WithIssuedAt
	// solo lo compara con now cuando está presente, pero no exige que exista.
	sessionID, err := uuid.Parse(claims.ID)
	if err != nil {
		return TokenClaims{}, invalidSession("jti inválido: %v", err)
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return TokenClaims{}, invalidSession("sub inválido: %v", err)
	}
	if claims.IssuedAt == nil {
		return TokenClaims{}, invalidSession("falta iat")
	}

	return TokenClaims{
		SessionID: sessionID,
		UserID:    userID,
		IssuedAt:  claims.IssuedAt.Time,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}

// invalidSession arma un error que envuelve domain.ErrInvalidSession con el detalle del rechazo.
// El detalle se formatea con %v para no exponer la cadena de errores interna de la librería.
func invalidSession(format string, args ...any) error {
	return fmt.Errorf("%w: %s", domain.ErrInvalidSession, fmt.Sprintf(format, args...))
}
