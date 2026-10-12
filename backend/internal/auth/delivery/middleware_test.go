package delivery

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/service"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
)

// Pruebas de integración de US3 (T055): montan el router real de la feature contra la base
// smye_test y cuelgan del grupo protegido que devuelve RegisterRoutes una acción protegida de
// prueba. Verifican que la acción solo se ejecuta con sesión vigente y atribuida a su titular, que
// registro e inicio de sesión siguen siendo públicos, y que la sesión ausente, vencida y adulterada
// reciben la misma respuesta 401 sin que la acción se ejecute ni parcialmente (FR-015, FR-016,
// FR-019, FR-020, FR-021).

// sessionRequiredMessage es el texto exacto del 401 de sesión inválida que fija el contrato
// (respuesta SesionInvalida de contracts/openapi.yaml).
const sessionRequiredMessage = "Se requiere iniciar sesión."

// protectedProbePath es la ruta de la acción protegida de prueba; no existe en producción.
const protectedProbePath = "/api/v1/solo-prueba-accion-protegida"

// protectedProbe registra cuántas veces se ejecutó la acción protegida de prueba y a quién se le
// atribuyó la última ejecución, para comprobar que una petición rechazada no la alcanza.
type protectedProbe struct {
	executions int
	attributed uuid.UUID
	hasOwner   bool
}

// protectedTestEnv reúne lo que necesita cada escenario: el router compuesto, la base de test, el
// secreto de firma de test y la sonda de la acción protegida.
type protectedTestEnv struct {
	router *gin.Engine
	db     *gorm.DB
	secret string
	probe  *protectedProbe
}

// newProtectedTestEnv abre la base de test vacía, compone la cadena real repositorios, servicio,
// manejadores y rutas igual que cmd/app, y cuelga del grupo protegido la acción de prueba. La
// acción anota su ejecución y el titular que el middleware dejó en el contexto, y responde 200.
func newProtectedTestEnv(t *testing.T) protectedTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cfg := testsupport.OpenDatabase(t)
	auth := service.NewAuthService(
		repository.NewUserRepository(db),
		repository.NewSessionRepository(db),
		repository.NewAuthEventRepository(db),
		cfg.JWTSecret,
	)
	router := gin.New()
	protected := RegisterRoutes(router, NewHandler(auth))

	probe := &protectedProbe{}
	protected.POST("/solo-prueba-accion-protegida", func(c *gin.Context) {
		probe.executions++
		probe.attributed, probe.hasOwner = userIDFromContext(c)
		c.Status(http.StatusOK)
	})
	return protectedTestEnv{router: router, db: db, secret: cfg.JWTSecret, probe: probe}
}

// callProtected invoca la acción protegida con la cabecera Authorization indicada; si es vacía, la
// petición sale sin cabecera, como la de un visitante sin sesión.
func callProtected(t *testing.T, router *gin.Engine, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, protectedProbePath, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// loginToken inicia sesión con la cuenta sembrada de Ana y devuelve el token emitido.
func loginToken(t *testing.T, router *gin.Engine) string {
	t.Helper()
	rec := postLogin(t, router, anaEmail, anaPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("el inicio de sesión de preparación devolvió %d; cuerpo: %s", rec.Code, rec.Body.String())
	}
	var body loginResponseBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Token == "" {
		t.Fatalf("el inicio de sesión de preparación no devolvió un token: %v; cuerpo: %s", err, rec.Body.String())
	}
	return body.Token
}

// requireSameAsNoSession exige que la respuesta sea el 401 del contrato con solo
// {"error": "Se requiere iniciar sesión."} y que sea idéntica (estado, tipo de contenido y cuerpo)
// a la que recibe una petición sin sesión, para que el rechazo no revele el motivo (FR-015, FR-016).
func requireSameAsNoSession(t *testing.T, env protectedTestEnv, rec *httptest.ResponseRecorder) {
	t.Helper()
	requireErrorResponse(t, rec, http.StatusUnauthorized, sessionRequiredMessage)

	baseline := callProtected(t, env.router, "")
	if rec.Code != baseline.Code ||
		rec.Header().Get("Content-Type") != baseline.Header().Get("Content-Type") ||
		rec.Body.String() != baseline.Body.String() {
		t.Errorf("la respuesta (%d, %q, %s) difiere de la de sesión ausente (%d, %q, %s)",
			rec.Code, rec.Header().Get("Content-Type"), rec.Body.String(),
			baseline.Code, baseline.Header().Get("Content-Type"), baseline.Body.String())
	}
}

// requireNotExecuted exige que la acción protegida no se haya ejecutado más veces que las
// esperadas y que la petición rechazada no haya creado sesiones ni registrado eventos (FR-020).
func requireNotExecuted(t *testing.T, env protectedTestEnv, wantExecutions int, sessionsBefore, eventsBefore int64) {
	t.Helper()
	if env.probe.executions != wantExecutions {
		t.Errorf("la acción protegida se ejecutó %d veces; se esperaban %d: una petición rechazada no debe alcanzarla (FR-020)",
			env.probe.executions, wantExecutions)
	}
	if n := countRows(t, env.db, &domain.Session{}, ""); n != sessionsBefore {
		t.Errorf("hay %d sesiones después del rechazo; se esperaban %d", n, sessionsBefore)
	}
	if n := countRows(t, env.db, &domain.AuthEvent{}, ""); n != eventsBefore {
		t.Errorf("hay %d eventos después del rechazo; se esperaban %d", n, eventsBefore)
	}
}

// requireRejected combina las dos comprobaciones de un rechazo: respuesta idéntica a la de sesión
// ausente y acción protegida sin ejecutar ni dejar efectos.
func requireRejected(t *testing.T, env protectedTestEnv, authorization string, wantExecutions int) {
	t.Helper()
	sessionsBefore := countRows(t, env.db, &domain.Session{}, "")
	eventsBefore := countRows(t, env.db, &domain.AuthEvent{}, "")
	rec := callProtected(t, env.router, authorization)
	requireSameAsNoSession(t, env, rec)
	requireNotExecuted(t, env, wantExecutions, sessionsBefore, eventsBefore)
}

// insertSession guarda directamente en sessions una sesión del usuario con los momentos indicados
// y devuelve un token firmado con el secreto de test que la referencia por su jti.
func insertSession(t *testing.T, env protectedTestEnv, userID uuid.UUID, createdAt, expiresAt time.Time) string {
	t.Helper()
	session := domain.Session{ID: uuid.New(), UserID: userID, CreatedAt: createdAt, ExpiresAt: expiresAt}
	if err := env.db.Create(&session).Error; err != nil {
		t.Fatalf("no se pudo preparar la sesión: %v", err)
	}
	token, err := service.IssueToken(env.secret, session)
	if err != nil {
		t.Fatalf("no se pudo firmar el token de preparación: %v", err)
	}
	return token
}

// tamperSubject reescribe el claim sub del payload de un JWT con otro identificador y conserva la
// firma original, como haría quien intenta hacerse pasar por otra persona.
func tamperSubject(t *testing.T, token string, sub uuid.UUID) string {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("el token de preparación no tiene tres partes: %q", token)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("no se pudo decodificar el payload: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatalf("el payload no es JSON: %v", err)
	}
	claims["sub"] = sub.String()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("no se pudo serializar el payload adulterado: %v", err)
	}
	parts[1] = base64.RawURLEncoding.EncodeToString(payload)
	return strings.Join(parts, ".")
}

// Escenario: Acceso con sesión vigente (001/US3)
// Dado un usuario con sesión vigente, cuando accede a una función protegida, entonces la función se
// ejecuta y la acción queda atribuida a ese usuario (claim sub, FR-021, RN-09).
func TestUS3_AccesoConSesionVigente(t *testing.T) {
	env := newProtectedTestEnv(t)
	user := seedAccount(t, env.db)
	token := loginToken(t, env.router)

	rec := callProtected(t, env.router, "Bearer "+token)

	if rec.Code != http.StatusOK {
		t.Fatalf("estado %d, se esperaba %d; cuerpo: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if env.probe.executions != 1 {
		t.Errorf("la acción protegida se ejecutó %d veces; se esperaba 1", env.probe.executions)
	}
	if !env.probe.hasOwner || env.probe.attributed != user.ID {
		t.Errorf("la acción se atribuyó a (%v, %v); se esperaba (%s, true) (FR-021, RN-09)",
			env.probe.attributed, env.probe.hasOwner, user.ID)
	}
}

// Escenario: Rutas públicas (001/US3)
// Dado un visitante sin sesión, cuando accede al registro o al inicio de sesión, entonces puede
// usarlos con normalidad: son las únicas funciones públicas (FR-019, RN-08).
func TestUS3_RutasPublicas(t *testing.T) {
	env := newProtectedTestEnv(t)

	t.Run("el registro funciona sin sesión", func(t *testing.T) {
		rec := postRegister(t, env.router, validRegistration())
		if rec.Code != http.StatusCreated {
			t.Errorf("POST %s sin sesión devolvió %d; se esperaba %d; cuerpo: %s",
				registerPath, rec.Code, http.StatusCreated, rec.Body.String())
		}
	})

	t.Run("el inicio de sesión funciona sin sesión", func(t *testing.T) {
		rec := postLogin(t, env.router, anaEmail, anaPassword)
		if rec.Code != http.StatusOK {
			t.Errorf("POST %s sin sesión devolvió %d; se esperaba %d; cuerpo: %s",
				loginPath, rec.Code, http.StatusOK, rec.Body.String())
		}
	})

	t.Run("cualquier otra función exige sesión", func(t *testing.T) {
		requireRejected(t, env, "", 0)
	})
}

// Escenario: Sesión vencida (001/US3)
// Dado un usuario que está operando en la aplicación, cuando su sesión vence entre una acción y la
// siguiente, entonces la siguiente acción se rechaza de forma explícita indicando que debe volver a
// iniciar sesión, sin ejecutarse a medias ni fallar en silencio (FR-016, FR-020, RN-07).
func TestUS3_SesionVencida(t *testing.T) {
	t.Run("la sesión vence entre una acción y la siguiente", func(t *testing.T) {
		env := newProtectedTestEnv(t)
		seedAccount(t, env.db)
		token := loginToken(t, env.router)

		// La primera acción ocurre con la sesión vigente y se ejecuta.
		if rec := callProtected(t, env.router, "Bearer "+token); rec.Code != http.StatusOK {
			t.Fatalf("la primera acción devolvió %d con la sesión vigente; cuerpo: %s", rec.Code, rec.Body.String())
		}

		// La sesión vence en la base antes de la segunda acción; el JWT conserva su exp original,
		// de modo que el rechazo depende de la fila de sessions (RN-07).
		if err := env.db.Model(&domain.Session{}).Where("user_id IS NOT NULL").
			Update("expires_at", time.Now().Add(-time.Second)).Error; err != nil {
			t.Fatalf("no se pudo vencer la sesión: %v", err)
		}

		requireRejected(t, env, "Bearer "+token, 1)
	})

	t.Run("token cuyo vencimiento ya pasó", func(t *testing.T) {
		env := newProtectedTestEnv(t)
		user := seedAccount(t, env.db)
		now := time.Now()
		token := insertSession(t, env, user.ID, now.Add(-9*time.Hour), now.Add(-time.Hour))

		requireRejected(t, env, "Bearer "+token, 0)
	})
}

// Escenario: Sesión adulterada (001/US3)
// Dado un intento de acceso con una sesión adulterada o inventada, cuando se solicita una función
// protegida, entonces el sistema la trata exactamente igual que a la ausencia de sesión y rechaza
// el acceso (FR-015, FR-020).
func TestUS3_SesionAdulterada(t *testing.T) {
	env := newProtectedTestEnv(t)
	user := seedAccount(t, env.db)
	token := loginToken(t, env.router)
	now := time.Now()

	// Token firmado con otro secreto para una sesión con datos plausibles.
	foreignToken, err := service.IssueToken(strings.Repeat("x", 32)+"-otro-secreto",
		domain.Session{ID: uuid.New(), UserID: user.ID, CreatedAt: now, ExpiresAt: now.Add(sessionLifetime)})
	if err != nil {
		t.Fatalf("no se pudo firmar el token ajeno: %v", err)
	}

	// Token bien firmado cuyo jti no corresponde a ninguna fila de sessions (sesión inventada).
	inventedToken, err := service.IssueToken(env.secret,
		domain.Session{ID: uuid.New(), UserID: user.ID, CreatedAt: now, ExpiresAt: now.Add(sessionLifetime)})
	if err != nil {
		t.Fatalf("no se pudo firmar el token inventado: %v", err)
	}

	cases := []struct {
		name          string
		authorization string
	}{
		{"firma alterada", "Bearer " + token[:len(token)-2] + flipLastChars(token[len(token)-2:])},
		{"payload alterado con la firma original", "Bearer " + tamperSubject(t, token, uuid.New())},
		{"firmado con otro secreto", "Bearer " + foreignToken},
		{"sesión inventada", "Bearer " + inventedToken},
		{"texto que no es un JWT", "Bearer no-es-un-jwt"},
		{"esquema distinto de Bearer", "Basic " + token},
		{"Bearer sin token", "Bearer "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireRejected(t, env, tc.authorization, 0)
		})
	}
}

// flipLastChars reemplaza cada carácter base64url por otro distinto para invalidar la firma sin
// romper el formato del token.
func flipLastChars(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] == 'A' {
			b[i] = 'B'
		} else {
			b[i] = 'A'
		}
	}
	return string(b)
}
