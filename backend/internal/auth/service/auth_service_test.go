package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// Credenciales de la cuenta de los escenarios de US2 en spec.md.
const (
	knownEmail    = "ana@example.com"
	knownPassword = "Proyecto2026"
	unknownEmail  = "nadie@example.com"
	wrongPassword = "Incorrecta2026"
)

// errNotUsed señala una operación de repositorio que el inicio de sesión no debería invocar.
var errNotUsed = errors.New("operación no usada por el inicio de sesión")

// fakeUserRepository guarda cuentas en memoria por correo normalizado y respeta el contrato de
// domain.UserRepository: un correo inexistente devuelve ErrInvalidCredentials, nunca (nil, nil).
type fakeUserRepository struct {
	byEmail map[string]domain.User
}

func (r *fakeUserRepository) Create(context.Context, *domain.User, *domain.AuthEvent) error {
	return errNotUsed
}

func (r *fakeUserRepository) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	user, ok := r.byEmail[email]
	if !ok {
		return nil, domain.ErrInvalidCredentials
	}
	return &user, nil
}

func (r *fakeUserRepository) FindByID(context.Context, uuid.UUID) (*domain.User, error) {
	return nil, errNotUsed
}

// fakeSessionRepository registra en memoria las sesiones creadas para verificar si el inicio de
// sesión emitió o no una sesión.
type fakeSessionRepository struct {
	created []domain.Session
}

func (r *fakeSessionRepository) Create(_ context.Context, session *domain.Session, _ *domain.AuthEvent) error {
	r.created = append(r.created, *session)
	return nil
}

func (r *fakeSessionRepository) FindByID(context.Context, uuid.UUID) (*domain.Session, error) {
	return nil, errNotUsed
}

func (r *fakeSessionRepository) Revoke(context.Context, uuid.UUID, time.Time, *domain.AuthEvent) error {
	return errNotUsed
}

// fakeAuthEventRepository acepta los eventos aislados (login_failed) sin persistirlos.
type fakeAuthEventRepository struct {
	created []domain.AuthEvent
}

func (r *fakeAuthEventRepository) Create(_ context.Context, event *domain.AuthEvent) error {
	r.created = append(r.created, *event)
	return nil
}

// hashForTest genera el hash bcrypt de la contraseña directamente con la biblioteca, sin pasar por
// el código de producción. Usa el coste mínimo porque la comparación no depende del coste y así la
// prueba es rápida.
func hashForTest(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return string(hash)
}

// loginFixture arma el servicio con la cuenta ana@example.com registrada y repositorios en memoria.
type loginFixture struct {
	service  *AuthService
	user     domain.User
	sessions *fakeSessionRepository
}

func newLoginFixture(t *testing.T) loginFixture {
	t.Helper()
	user := domain.User{
		ID:           uuid.New(),
		FullName:     "Ana Pérez",
		Email:        knownEmail,
		PasswordHash: hashForTest(t, knownPassword),
	}
	users := &fakeUserRepository{byEmail: map[string]domain.User{knownEmail: user}}
	sessions := &fakeSessionRepository{}
	events := &fakeAuthEventRepository{}
	return loginFixture{
		service:  NewAuthService(users, sessions, events, testJWTSecret),
		user:     user,
		sessions: sessions,
	}
}

// assertInvalidCredentials verifica que un intento fallido devuelva el error genérico de
// credenciales inválidas, sin token, sin sesión emitida y sin el correo ni la contraseña en el
// mensaje (RN-06, FR-011).
func assertInvalidCredentials(t *testing.T, f loginFixture, result LoginResult, err error, email, password string) {
	t.Helper()
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidCredentials), "se esperaba ErrInvalidCredentials, se obtuvo %v", err)
	assert.Empty(t, result.Token, "un intento fallido no debería devolver token")
	assert.Empty(t, f.sessions.created, "un intento fallido no debería crear sesión")
	assert.NotContains(t, err.Error(), email, "el error no debería revelar el correo intentado")
	assert.NotContains(t, err.Error(), password, "el error no debería contener la contraseña")
}

// La comparación bcrypt acepta la contraseña con la que se generó el hash (FR-012).
func TestComparePassword_AcceptsMatchingPassword(t *testing.T) {
	hash := hashForTest(t, knownPassword)

	assert.NoError(t, ComparePassword(hash, knownPassword))
}

// La comparación bcrypt rechaza con el error genérico de credenciales cualquier contraseña distinta
// de la original, incluidas las que solo difieren en mayúsculas o en espacios (RN-06).
func TestComparePassword_RejectsDifferentPassword(t *testing.T) {
	hash := hashForTest(t, knownPassword)

	for name, candidate := range map[string]string{
		"otra contraseña":             wrongPassword,
		"distinta capitalización":     "proyecto2026",
		"un dígito distinto":          "Proyecto2027",
		"espacio al final":            knownPassword + " ",
		"prefijo de la contraseña":    "Proyecto202",
		"contraseña vacía":            "",
		"el hash como si fuera clave": hash,
	} {
		t.Run(name, func(t *testing.T) {
			err := ComparePassword(hash, candidate)
			assert.True(t, errors.Is(err, domain.ErrInvalidCredentials), "se esperaba ErrInvalidCredentials, se obtuvo %v", err)
		})
	}
}

// Con correo y contraseña correctos el inicio de sesión supera la comparación bcrypt y emite una
// sesión del titular. Evita que un Login que rechace siempre satisfaga las pruebas de RN-06.
func TestLogin_CorrectCredentialsIssueSession(t *testing.T) {
	f := newLoginFixture(t)

	result, err := f.service.Login(context.Background(), knownEmail, knownPassword)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Token, "el inicio exitoso debería devolver el token")
	assert.False(t, result.ExpiresAt.IsZero(), "el inicio exitoso debería devolver el vencimiento")
	require.Len(t, f.sessions.created, 1, "el inicio exitoso debería crear una sesión")
	assert.Equal(t, f.user.ID, f.sessions.created[0].UserID, "la sesión debería pertenecer al titular")
}

// Escenario: Caso de error — correo inexistente con cualquier contraseña (001/US2)
// RN-06: el rechazo es el error genérico de credenciales inválidas y no emite sesión.
func TestUS2_CorreoInexistente(t *testing.T) {
	for name, password := range map[string]string{
		"con la contraseña de otra cuenta": knownPassword,
		"con una contraseña cualquiera":    wrongPassword,
	} {
		t.Run(name, func(t *testing.T) {
			f := newLoginFixture(t)

			result, err := f.service.Login(context.Background(), unknownEmail, password)
			assertInvalidCredentials(t, f, result, err, unknownEmail, password)
		})
	}
}

// Escenario: Caso de error — contraseña incorrecta con el mismo mensaje del correo inexistente (001/US2)
// RN-06, FR-014, SC-004: el error es idéntico al del correo inexistente, de modo que no revela que
// el correo sí está registrado.
func TestUS2_ContrasenaIncorrecta(t *testing.T) {
	f := newLoginFixture(t)

	result, err := f.service.Login(context.Background(), knownEmail, wrongPassword)
	assertInvalidCredentials(t, f, result, err, knownEmail, wrongPassword)

	// Se repite el camino del correo inexistente con la misma contraseña y se comparan los errores.
	_, unknownErr := newLoginFixture(t).service.Login(context.Background(), unknownEmail, wrongPassword)
	require.Error(t, err)
	require.Error(t, unknownErr)
	assert.Equal(t, unknownErr.Error(), err.Error(), "el mensaje debería ser idéntico al del correo inexistente")
	assert.True(t, errors.Is(err, domain.ErrInvalidCredentials) && errors.Is(unknownErr, domain.ErrInvalidCredentials),
		"ambos caminos deberían devolver ErrInvalidCredentials; contraseña incorrecta: %v, correo inexistente: %v", err, unknownErr)
}
