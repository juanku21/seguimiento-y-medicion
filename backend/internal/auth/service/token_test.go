package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// Secretos de prueba con al menos 32 bytes, como exige JWT_SECRET (research.md sección 2).
const (
	testJWTSecret  = "secreto-de-prueba-de-al-menos-32-bytes"
	otherJWTSecret = "otro-secreto-de-prueba-de-al-menos-32b"
)

// sessionDuration es el plazo fijo de una sesión que fija la spec (FR-018).
const sessionDuration = 8 * time.Hour

// newTestSession arma una sesión emitida en issuedAt con vencimiento a las 8 horas, igual que la
// fila de sessions que crea el inicio de sesión (data-model.md sección 2). El instante se trunca a
// segundos porque los claims numéricos del JWT no tienen más precisión.
func newTestSession(issuedAt time.Time) domain.Session {
	createdAt := issuedAt.UTC().Truncate(time.Second)
	return domain.Session{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		ExpiresAt: createdAt.Add(sessionDuration),
	}
}

// issueTestToken emite el token de la sesión y corta la prueba si la emisión falla.
func issueTestToken(t *testing.T, session domain.Session) string {
	t.Helper()
	token, err := IssueToken(testJWTSecret, session)
	require.NoError(t, err, "la emisión del token no debería fallar")
	require.NotEmpty(t, token, "la emisión debería devolver un token")
	return token
}

// La emisión firma con HS256 y el secreto de JWT_SECRET: un verificador independiente, que solo
// acepta HS256 y usa ese secreto, debe validar el token y leer los cuatro claims de la sesión.
func TestIssueToken_SignsWithHS256AndSessionClaims(t *testing.T) {
	session := newTestSession(time.Now())
	token := issueTestToken(t, session)

	claims := jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) {
		return []byte(testJWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithTimeFunc(func() time.Time {
		return session.CreatedAt.Add(time.Minute)
	}))
	require.NoError(t, err, "el token debería verificarse con HS256 y el secreto de JWT_SECRET")
	assert.Equal(t, jwt.SigningMethodHS256.Alg(), parsed.Method.Alg())

	assert.Equal(t, session.ID.String(), claims.ID, "jti debería ser sessions.id")
	assert.Equal(t, session.UserID.String(), claims.Subject, "sub debería ser users.id")
	require.NotNil(t, claims.ExpiresAt, "el token debería llevar exp")
	require.NotNil(t, claims.IssuedAt, "el token debería llevar iat")
	assert.True(t, session.ExpiresAt.Equal(claims.ExpiresAt.Time), "exp debería ser sessions.expires_at")
	assert.True(t, session.CreatedAt.Equal(claims.IssuedAt.Time), "iat debería ser sessions.created_at")
}

// El parseo de un token válido devuelve los mismos datos de la sesión que lo originó.
func TestParseToken_ReturnsSessionClaims(t *testing.T) {
	session := newTestSession(time.Now())
	token := issueTestToken(t, session)

	claims, err := ParseToken(testJWTSecret, token, session.CreatedAt.Add(time.Minute))
	require.NoError(t, err)
	assert.Equal(t, session.ID, claims.SessionID)
	assert.Equal(t, session.UserID, claims.UserID)
	assert.True(t, session.CreatedAt.Equal(claims.IssuedAt), "IssuedAt debería ser sessions.created_at")
	assert.True(t, session.ExpiresAt.Equal(claims.ExpiresAt), "ExpiresAt debería ser sessions.expires_at")
}

// El vencimiento es de 8 horas fijas desde la emisión (FR-018): el token sirve hasta un segundo
// antes y se rechaza como sesión inválida desde el instante exacto del vencimiento.
func TestParseToken_ExpiresEightHoursAfterIssue(t *testing.T) {
	session := newTestSession(time.Now())
	token := issueTestToken(t, session)

	t.Run("vigente un segundo antes de las 8 horas", func(t *testing.T) {
		claims, err := ParseToken(testJWTSecret, token, session.CreatedAt.Add(sessionDuration-time.Second))
		require.NoError(t, err)
		assert.Equal(t, sessionDuration, claims.ExpiresAt.Sub(claims.IssuedAt), "exp debería quedar a 8 horas de iat")
	})

	// Variantes de un token ya vencido: en el instante exacto y después del vencimiento.
	for name, offset := range map[string]time.Duration{
		"vencido a las 8 horas exactas":     sessionDuration,
		"vencido un segundo después":        sessionDuration + time.Second,
		"vencido un día después de emitido": 24 * time.Hour,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseToken(testJWTSecret, token, session.CreatedAt.Add(offset))
			assert.True(t, errors.Is(err, domain.ErrInvalidSession), "se esperaba ErrInvalidSession, se obtuvo %v", err)
		})
	}
}

// El vencimiento no se renueva por actividad (FR-018): usar el token varias veces durante la sesión
// no corre su exp, y a las 8 horas desde la emisión se rechaza igual.
func TestParseToken_DoesNotRenewOnActivity(t *testing.T) {
	session := newTestSession(time.Now())
	token := issueTestToken(t, session)

	// Usos sucesivos del mismo token a lo largo de la sesión.
	for _, offset := range []time.Duration{time.Minute, 2 * time.Hour, 7 * time.Hour, 7*time.Hour + 59*time.Minute} {
		claims, err := ParseToken(testJWTSecret, token, session.CreatedAt.Add(offset))
		require.NoError(t, err, "el token debería ser válido a %v de la emisión", offset)
		assert.True(t, session.ExpiresAt.Equal(claims.ExpiresAt), "el uso a %v no debería mover el vencimiento", offset)
	}

	_, err := ParseToken(testJWTSecret, token, session.CreatedAt.Add(sessionDuration))
	assert.True(t, errors.Is(err, domain.ErrInvalidSession), "tras la actividad el token debería vencer igual a las 8 horas, se obtuvo %v", err)
}

// Un token con firma inválida se rechaza como sesión inválida (FR-015): firmado con otro secreto,
// con el contenido alterado después de firmar, firmado con otro algoritmo o sin firma.
func TestParseToken_RejectsInvalidSignature(t *testing.T) {
	session := newTestSession(time.Now())
	now := session.CreatedAt.Add(time.Minute)
	claims := jwt.RegisteredClaims{
		ID:        session.ID.String(),
		Subject:   session.UserID.String(),
		IssuedAt:  jwt.NewNumericDate(session.CreatedAt),
		ExpiresAt: jwt.NewNumericDate(session.ExpiresAt),
	}

	// signWith firma los claims de la sesión con el método y la clave indicados.
	signWith := func(t *testing.T, method jwt.SigningMethod, key any) string {
		t.Helper()
		token, err := jwt.NewWithClaims(method, claims).SignedString(key)
		require.NoError(t, err)
		return token
	}

	// tamperPayload reemplaza el payload de un token emitido por el de otro usuario, sin volver a
	// firmar, como haría alguien que intenta suplantar a otra cuenta.
	tamperPayload := func(t *testing.T) string {
		t.Helper()
		original := issueTestToken(t, session)
		forged := claims
		forged.Subject = uuid.New().String()
		forgedToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, forged).SignedString([]byte(otherJWTSecret))
		require.NoError(t, err)
		originalParts := strings.Split(original, ".")
		forgedParts := strings.Split(forgedToken, ".")
		require.Len(t, originalParts, 3)
		require.Len(t, forgedParts, 3)
		return strings.Join([]string{originalParts[0], forgedParts[1], originalParts[2]}, ".")
	}

	cases := map[string]func(t *testing.T) string{
		"firmado con otro secreto": func(t *testing.T) string {
			return signWith(t, jwt.SigningMethodHS256, []byte(otherJWTSecret))
		},
		"contenido alterado después de firmar": tamperPayload,
		"firmado con HS512 en lugar de HS256": func(t *testing.T) string {
			return signWith(t, jwt.SigningMethodHS512, []byte(testJWTSecret))
		},
		"sin firma (alg none)": func(t *testing.T) string {
			return signWith(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType)
		},
		"texto que no es un JWT": func(t *testing.T) string {
			return "esto-no-es-un-token"
		},
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseToken(testJWTSecret, build(t), now)
			assert.True(t, errors.Is(err, domain.ErrInvalidSession), "se esperaba ErrInvalidSession, se obtuvo %v", err)
		})
	}
}
