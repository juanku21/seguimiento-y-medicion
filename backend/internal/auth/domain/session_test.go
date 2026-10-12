package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// Pruebas unitarias obligatorias de la regla de negocio RN-07 (data-model.md sección 2): una
// sesión está vigente solo si revoked_at es nulo y expires_at es posterior al instante de
// verificación; CERRADA y VENCIDA son estados terminales que no vuelven a ser vigentes. Los
// nombres llevan el prefijo "Dominio" para no chocar con las pruebas de integración de
// middleware_test.go (T055), que cubren los mismos escenarios de punta a punta con los nombres
// TestUS3_AccesoConSesionVigente y TestUS3_SesionVencida.

// newSessionAt arma una fila de sessions con expires_at fijo a 8 horas desde createdAt, igual que
// el inicio de sesión real (FR-013, FR-018), y el revoked_at que indique cada caso.
func newSessionAt(createdAt time.Time, revokedAt *time.Time) Session {
	return Session{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		ExpiresAt: createdAt.Add(SessionLifetime),
		RevokedAt: revokedAt,
	}
}

// Escenario: Acceso con sesión vigente (001/US3)
// Una sesión sin revocar y con vencimiento futuro es vigente, tanto recién emitida como un instante
// antes de vencer.
func TestUS3_DominioSesionVigente(t *testing.T) {
	createdAt := time.Now()
	session := newSessionAt(createdAt, nil)

	t.Run("recién emitida", func(t *testing.T) {
		if !session.IsActive(createdAt) {
			t.Errorf("IsActive(%v) = false; una sesión recién emitida debería ser vigente", createdAt)
		}
	})

	t.Run("un instante antes de vencer", func(t *testing.T) {
		now := session.ExpiresAt.Add(-time.Nanosecond)
		if !session.IsActive(now) {
			t.Errorf("IsActive(%v) = false; la sesión debería seguir vigente antes de vencer", now)
		}
	})
}

// Escenario: Sesión vence entre una acción y la siguiente (001/US3)
// Al llegar expires_at la sesión deja de ser vigente (VENCIDA) de forma estricta
// (expires_at > now, no >=) y ese estado es terminal: por más que pase el tiempo, no vuelve a ser
// vigente.
func TestUS3_DominioSesionVencidaEsTerminal(t *testing.T) {
	createdAt := time.Now()
	session := newSessionAt(createdAt, nil)

	// Control: la misma sesión es vigente un instante antes de expires_at, para aislar que el
	// rechazo de los casos siguientes se debe al vencimiento y no a un error de construcción.
	if !session.IsActive(session.ExpiresAt.Add(-time.Nanosecond)) {
		t.Fatalf("control: la sesión debería ser vigente un instante antes de expires_at")
	}

	t.Run("deja de ser vigente exactamente al vencer", func(t *testing.T) {
		if session.IsActive(session.ExpiresAt) {
			t.Errorf("IsActive(expires_at) = true; la regla exige expires_at > now, no >=")
		}
	})

	t.Run("sigue sin ser vigente mucho después de vencer", func(t *testing.T) {
		if session.IsActive(session.ExpiresAt.Add(30 * 24 * time.Hour)) {
			t.Errorf("una sesión vencida no debería volver a ser vigente con el paso del tiempo")
		}
	})
}

// Escenario: Sesión cerrada es un estado terminal (001/US3, data-model.md sección 2)
// Revocar la sesión la deja CERRADA incluso si expires_at todavía no llegó, y ese rechazo no se
// revierte con el paso del tiempo dentro del plazo de vigencia original.
func TestUS3_DominioSesionCerradaEsTerminal(t *testing.T) {
	createdAt := time.Now()
	revokedAt := createdAt.Add(time.Hour)
	now := revokedAt.Add(time.Minute)

	// Control: una sesión equivalente sin revocar sigue vigente en el mismo instante, para aislar
	// que el rechazo se debe a revoked_at y no a expires_at.
	active := newSessionAt(createdAt, nil)
	if !active.IsActive(now) {
		t.Fatalf("control: una sesión sin revocar en el mismo instante debería ser vigente")
	}
	if !now.Before(active.ExpiresAt) {
		t.Fatalf("la prueba requiere now anterior a expires_at para aislar el efecto de revoked_at")
	}

	closed := newSessionAt(createdAt, &revokedAt)
	if closed.IsActive(now) {
		t.Errorf("IsActive(%v) = true tras revocar; una sesión CERRADA no debería ser vigente", now)
	}
}
