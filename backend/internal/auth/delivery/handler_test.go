package delivery

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/service"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/httpserver"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
)

// Pruebas de integración de punta a punta de la feature de autenticación: atraviesan el router de
// Gin, el servicio y el repositorio contra la base smye_test (Principio II y V). Las de US1 (T027)
// cubren POST /api/v1/auth/register; las historias siguientes agregan las suyas en este archivo.

// registerPath es la ruta pública de registro según contracts/openapi.yaml.
const registerPath = "/api/v1/auth/register"

// emailUnavailableMessage es el texto exacto del 409 que fija el contrato y T034.
const emailUnavailableMessage = "El correo electrónico no está disponible."

// validationErrorBody refleja ValidationErrorResponse del contrato; se declara en la prueba para no
// depender de los DTO de producción al leer la respuesta.
type validationErrorBody struct {
	Error  string `json:"error"`
	Fields []struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	} `json:"fields"`
}

// newTestRouter abre la base de test vacía y compone la cadena real repositorios, servicio,
// manejadores y rutas, igual que cmd/app: con el límite global del cuerpo, sin CORS ni Swagger.
func newTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	router, db, _ := newTestRouterWithSecret(t)
	return router, db
}

// newTestRouterWithSecret es newTestRouter que además devuelve el JWT_SECRET de backend/.env.test
// con el que se compuso el servicio, para que las pruebas verifiquen la firma de los tokens.
func newTestRouterWithSecret(t *testing.T) (*gin.Engine, *gorm.DB, string) {
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
	router.Use(httpserver.LimitRequestBody())
	RegisterRoutes(router, NewHandler(auth))
	return router, db, cfg.JWTSecret
}

// validRegistration devuelve el cuerpo del escenario normal de US1; cada prueba modifica o quita
// solo el campo que le interesa.
func validRegistration() map[string]any {
	return map[string]any{
		"fullName": "Ana Pérez",
		"email":    "ana@example.com",
		"password": "Proyecto2026",
	}
}

// newRegisterRequest arma la solicitud POST de registro con el cuerpo serializado como JSON.
func newRegisterRequest(t *testing.T, body map[string]any) *http.Request {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("no se pudo serializar la solicitud: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, registerPath, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// postRegister envía el cuerpo a la ruta de registro y devuelve la respuesta grabada.
func postRegister(t *testing.T, router *gin.Engine, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, newRegisterRequest(t, body))
	return rec
}

// requireCreatedProfile exige un 201 cuyo cuerpo sea un UserProfile con id UUID, el nombre y el
// correo esperados, sin ningún campo ni valor de contraseña (FR-011), y devuelve el id.
func requireCreatedProfile(t *testing.T, rec *httptest.ResponseRecorder, password, wantName, wantEmail string) uuid.UUID {
	t.Helper()
	if rec.Code != http.StatusCreated {
		t.Fatalf("estado %d, se esperaba %d; cuerpo: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), password) {
		t.Errorf("la respuesta contiene la contraseña en claro: %s", rec.Body.String())
	}

	// El perfil se lee como mapa para detectar cualquier campo de más, además de los tres esperados.
	var profile map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &profile); err != nil {
		t.Fatalf("el cuerpo del 201 no es JSON válido: %v; cuerpo: %s", err, rec.Body.String())
	}
	for key := range profile {
		if key != "id" && key != "fullName" && key != "email" {
			t.Errorf("el perfil tiene el campo inesperado %q (solo id, fullName y email)", key)
		}
	}
	if profile["fullName"] != wantName {
		t.Errorf("fullName = %v, se esperaba %q", profile["fullName"], wantName)
	}
	if profile["email"] != wantEmail {
		t.Errorf("email = %v, se esperaba %q", profile["email"], wantEmail)
	}
	rawID, _ := profile["id"].(string)
	id, err := uuid.Parse(rawID)
	if err != nil {
		t.Fatalf("id = %v no es un UUID: %v", profile["id"], err)
	}
	return id
}

// requireStoredUser exige que la cuenta exista en users con el id, el nombre y el correo
// normalizado esperados, y que la contraseña solo esté como hash (FR-010).
func requireStoredUser(t *testing.T, db *gorm.DB, id uuid.UUID, password, wantName, wantEmail string) {
	t.Helper()
	var user domain.User
	if err := db.First(&user, "id = ?", id).Error; err != nil {
		t.Fatalf("no se encontró la cuenta %s en users: %v", id, err)
	}
	if user.FullName != wantName || user.Email != wantEmail {
		t.Errorf("cuenta guardada (%q, %q), se esperaba (%q, %q)", user.FullName, user.Email, wantName, wantEmail)
	}
	if user.PasswordHash == "" || user.PasswordHash == password {
		t.Errorf("password_hash vacío o en texto plano")
	}
}

// requireAccountCreatedEvent exige exactamente un evento account_created en auth_events y que esté
// atribuido a la cuenta indicada (FR-028, FR-029, SC-009).
func requireAccountCreatedEvent(t *testing.T, db *gorm.DB, userID uuid.UUID) {
	t.Helper()
	var events []domain.AuthEvent
	if err := db.Where("event_type = ?", domain.EventAccountCreated).Find(&events).Error; err != nil {
		t.Fatalf("no se pudo consultar auth_events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("hay %d eventos account_created, se esperaba exactamente 1", len(events))
	}
	if events[0].UserID == nil || *events[0].UserID != userID {
		t.Errorf("account_created tiene user_id %v, se esperaba %s", events[0].UserID, userID)
	}
	if events[0].CreatedAt.IsZero() {
		t.Errorf("account_created no registra el momento del evento")
	}
}

// countRows cuenta las filas del modelo indicado, opcionalmente filtradas.
func countRows(t *testing.T, db *gorm.DB, model any, query string, args ...any) int64 {
	t.Helper()
	var n int64
	tx := db.Model(model)
	if query != "" {
		tx = tx.Where(query, args...)
	}
	if err := tx.Count(&n).Error; err != nil {
		t.Fatalf("no se pudo contar filas: %v", err)
	}
	return n
}

// Escenario: Alta exitosa (001/US1)
// Dado que no existe ana@example.com, el registro de Ana Pérez con Proyecto2026 crea la cuenta,
// confirma el alta con 201 y deja el evento account_created atribuido a la cuenta.
func TestUS1_AltaExitosa(t *testing.T) {
	router, db := newTestRouter(t)
	body := validRegistration()

	rec := postRegister(t, router, body)

	id := requireCreatedProfile(t, rec, "Proyecto2026", "Ana Pérez", "ana@example.com")
	requireStoredUser(t, db, id, "Proyecto2026", "Ana Pérez", "ana@example.com")
	requireAccountCreatedEvent(t, db, id)
}

// Escenario: Correo normalizado (001/US1)
// El correo "  Ana@Example.COM  " se guarda y se devuelve recortado y en minúsculas (FR-004).
func TestUS1_CorreoNormalizado(t *testing.T) {
	router, db := newTestRouter(t)
	body := validRegistration()
	body["email"] = "  Ana@Example.COM  "

	rec := postRegister(t, router, body)

	id := requireCreatedProfile(t, rec, "Proyecto2026", "Ana Pérez", "ana@example.com")
	requireStoredUser(t, db, id, "Proyecto2026", "Ana Pérez", "ana@example.com")
	requireAccountCreatedEvent(t, db, id)
}

// Escenario: Límites de contraseña (001/US1)
// Una contraseña de exactamente 8 y una de exactamente 72 caracteres, con letra y número, son
// válidas y crean la cuenta (FR-006).
func TestUS1_LimitesDeContrasena(t *testing.T) {
	cases := []struct {
		name     string
		password string
	}{
		{"exactamente 8 caracteres", "abcdefg1"},
		{"exactamente 72 caracteres", "A1" + strings.Repeat("a", 70)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, db := newTestRouter(t)
			body := validRegistration()
			body["password"] = tc.password

			rec := postRegister(t, router, body)

			id := requireCreatedProfile(t, rec, tc.password, "Ana Pérez", "ana@example.com")
			requireStoredUser(t, db, id, tc.password, "Ana Pérez", "ana@example.com")
			requireAccountCreatedEvent(t, db, id)
		})
	}
}

// Escenario: Límites de nombre (001/US1)
// Un nombre de exactamente 2 y uno de exactamente 100 caracteres crean la cuenta; el de 100 usa
// letras acentuadas para verificar que se cuentan caracteres y no bytes (FR-002).
func TestUS1_LimitesDeNombre(t *testing.T) {
	cases := []struct {
		name     string
		fullName string
	}{
		{"exactamente 2 caracteres", "Lu"},
		{"exactamente 100 caracteres", strings.Repeat("Ánà ", 24) + "Pére"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, db := newTestRouter(t)
			body := validRegistration()
			body["fullName"] = tc.fullName

			rec := postRegister(t, router, body)

			id := requireCreatedProfile(t, rec, "Proyecto2026", tc.fullName, "ana@example.com")
			requireStoredUser(t, db, id, "Proyecto2026", tc.fullName, "ana@example.com")
			requireAccountCreatedEvent(t, db, id)
		})
	}
}

// Escenario: Registro concurrente (001/US1)
// Diez solicitudes simultáneas con el mismo correo dejan exactamente una cuenta y un solo evento
// account_created; las nueve restantes reciben 409 (FR-007, SC-006).
func TestUS1_RegistroConcurrente(t *testing.T) {
	router, db := newTestRouter(t)
	const requests = 10

	// Las solicitudes se arman antes y todas las goroutines esperan la misma señal de salida para
	// maximizar la simultaneidad.
	start := make(chan struct{})
	recorders := make([]*httptest.ResponseRecorder, requests)
	var wg sync.WaitGroup
	for i := range requests {
		req := newRegisterRequest(t, validRegistration())
		recorders[i] = httptest.NewRecorder()
		wg.Go(func() {
			<-start
			router.ServeHTTP(recorders[i], req)
		})
	}
	close(start)
	wg.Wait()

	// Se clasifican las respuestas: una sola alta y el resto rechazadas por correo no disponible.
	var created []*httptest.ResponseRecorder
	conflicts := 0
	for _, rec := range recorders {
		switch rec.Code {
		case http.StatusCreated:
			created = append(created, rec)
		case http.StatusConflict:
			conflicts++
		default:
			t.Errorf("estado inesperado %d; cuerpo: %s", rec.Code, rec.Body.String())
		}
	}
	if len(created) != 1 || conflicts != requests-1 {
		t.Fatalf("respuestas: %d con 201 y %d con 409, se esperaba 1 y %d", len(created), conflicts, requests-1)
	}

	id := requireCreatedProfile(t, created[0], "Proyecto2026", "Ana Pérez", "ana@example.com")
	if n := countRows(t, db, &domain.User{}, "email = ?", "ana@example.com"); n != 1 {
		t.Errorf("hay %d cuentas con ana@example.com, se esperaba exactamente 1", n)
	}
	requireAccountCreatedEvent(t, db, id)
}

// Escenario: Registro no emite sesión (001/US1)
// Tras un alta exitosa la persona queda sin sesión: la respuesta no trae token, vencimiento,
// cookie ni cabecera de autorización, no se crea ninguna fila en sessions ni un evento
// login_succeeded (FR-009). Además, pedir el perfil propio sin sesión justo después del alta se
// rechaza con 401: el registro no deja la sesión iniciada.
func TestUS1_RegistroNoEmiteSesion(t *testing.T) {
	router, db := newTestRouter(t)

	rec := postRegister(t, router, validRegistration())

	id := requireCreatedProfile(t, rec, "Proyecto2026", "Ana Pérez", "ana@example.com")
	requireAccountCreatedEvent(t, db, id)
	for _, field := range []string{`"token"`, `"expiresAt"`} {
		if strings.Contains(rec.Body.String(), field) {
			t.Errorf("la respuesta del registro incluye %s: %s", field, rec.Body.String())
		}
	}
	if rec.Header().Get("Set-Cookie") != "" || rec.Header().Get("Authorization") != "" {
		t.Errorf("la respuesta del registro emite credenciales en cabeceras: %v", rec.Header())
	}
	if n := countRows(t, db, &domain.Session{}, ""); n != 0 {
		t.Errorf("el registro creó %d sesiones, se esperaba 0", n)
	}
	if n := countRows(t, db, &domain.AuthEvent{}, "event_type = ?", domain.EventLoginSucceeded); n != 0 {
		t.Errorf("el registro dejó %d eventos login_succeeded, se esperaba 0", n)
	}

	// Sin iniciar sesión, la función protegida del perfil rechaza el acceso tras el alta.
	requireErrorResponse(t, getProfile(t, router, profilePath, ""), http.StatusUnauthorized, sessionRequiredMessage)
}

// Escenario: Correo duplicado (001/US1)
// Con una cuenta ana@example.com ya existente, registrarse con ANA@example.com responde 409 con el
// mensaje del contrato, no crea una segunda cuenta ni registra un nuevo account_created (FR-005).
func TestUS1_CorreoDuplicado(t *testing.T) {
	router, db := newTestRouter(t)

	// La cuenta previa se inserta directo en la base para que la prueba dependa solo del rechazo.
	existing := domain.User{
		ID:           uuid.New(),
		FullName:     "Ana Pérez",
		Email:        "ana@example.com",
		PasswordHash: strings.Repeat("x", 60),
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("no se pudo preparar la cuenta existente: %v", err)
	}
	body := validRegistration()
	body["email"] = "ANA@example.com"

	rec := postRegister(t, router, body)

	if rec.Code != http.StatusConflict {
		t.Fatalf("estado %d, se esperaba %d; cuerpo: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("el cuerpo del 409 no es JSON válido: %v", err)
	}
	if resp.Error != emailUnavailableMessage {
		t.Errorf("error = %q, se esperaba %q", resp.Error, emailUnavailableMessage)
	}
	if n := countRows(t, db, &domain.User{}, ""); n != 1 {
		t.Errorf("hay %d cuentas, se esperaba 1", n)
	}
	if n := countRows(t, db, &domain.AuthEvent{}, "event_type = ?", domain.EventAccountCreated); n != 0 {
		t.Errorf("el registro rechazado dejó %d eventos account_created, se esperaba 0", n)
	}
}

// Escenario: Entrada inválida (001/US1)
// Cada entrada inválida responde 400 (nunca 500) con ValidationErrorResponse que indica el campo a
// corregir, sin repetir la contraseña, y no crea ninguna cuenta ni ningún evento (FR-008, FR-028).
func TestUS1_EntradaInvalida(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(body map[string]any)
		field  string
	}{
		{"nombre ausente", func(b map[string]any) { delete(b, "fullName") }, "fullName"},
		{"correo ausente", func(b map[string]any) { delete(b, "email") }, "email"},
		{"correo con formato inválido", func(b map[string]any) { b["email"] = "ana.example.com" }, "email"},
		{"contraseña de 7 caracteres", func(b map[string]any) { b["password"] = "abcdef1" }, "password"},
		{"contraseña de 73 caracteres", func(b map[string]any) { b["password"] = tooLongPassword }, "password"},
		{"contraseña de menos de 72 caracteres que supera los 72 bytes en UTF-8", func(b map[string]any) { b["password"] = multibytePassword }, "password"},
		{"contraseña sin letra", func(b map[string]any) { b["password"] = "12345678" }, "password"},
		{"contraseña sin dígito", func(b map[string]any) { b["password"] = "Proyectos" }, "password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, db := newTestRouter(t)
			body := validRegistration()
			tc.mutate(body)

			rec := postRegister(t, router, body)

			resp := requireValidationError(t, rec, body, tc.field)
			// La contraseña que excede los 72 bytes recibe el mismo mensaje que la de 73 caracteres.
			if tc.field == "password" && body["password"] == multibytePassword {
				ref := validRegistration()
				ref["password"] = tooLongPassword
				refResp := requireValidationError(t, postRegister(t, router, ref), ref, "password")
				if fieldMessage(resp, "password") != fieldMessage(refResp, "password") {
					t.Errorf("mensaje %q distinto del de 73 caracteres %q", fieldMessage(resp, "password"), fieldMessage(refResp, "password"))
				}
			}
			if n := countRows(t, db, &domain.User{}, ""); n != 0 {
				t.Errorf("se crearon %d cuentas, se esperaba 0", n)
			}
			if n := countRows(t, db, &domain.AuthEvent{}, ""); n != 0 {
				t.Errorf("se registraron %d eventos, se esperaba 0 (sin account_created)", n)
			}
		})
	}
}

// tooLongPassword tiene 73 caracteres ASCII, con letra y número.
var tooLongPassword = "A1" + strings.Repeat("a", 71)

// multibytePassword tiene 41 caracteres (40 letras acentuadas y un número) y 81 bytes en UTF-8.
var multibytePassword = strings.Repeat("á", 40) + "1"

// requireValidationError exige un 400 con ValidationErrorResponse no vacío que señale el campo
// indicado y que no repita la contraseña enviada (FR-011), y devuelve el cuerpo decodificado.
func requireValidationError(t *testing.T, rec *httptest.ResponseRecorder, sent map[string]any, field string) validationErrorBody {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("estado %d, se esperaba %d; cuerpo: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if password, ok := sent["password"].(string); ok && strings.Contains(rec.Body.String(), password) {
		t.Errorf("la respuesta de error repite la contraseña: %s", rec.Body.String())
	}
	var resp validationErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("el cuerpo del 400 no es JSON válido: %v; cuerpo: %s", err, rec.Body.String())
	}
	if resp.Error == "" {
		t.Errorf("el 400 no trae el mensaje general en error")
	}
	if fieldMessage(resp, field) == "" {
		t.Errorf("fields no señala %q con un mensaje; cuerpo: %s", field, rec.Body.String())
	}
	return resp
}

// fieldMessage devuelve el mensaje asociado al campo en la respuesta de validación, o vacío.
func fieldMessage(resp validationErrorBody, field string) string {
	for _, f := range resp.Fields {
		if f.Field == field {
			return f.Message
		}
	}
	return ""
}

// Pruebas de integración de US2 (T042): cubren POST /api/v1/auth/login de punta a punta, incluida la
// fila de sessions y los eventos login_succeeded y login_failed de auth_events (FR-029, SC-009).

// loginPath es la ruta pública de inicio de sesión según contracts/openapi.yaml.
const loginPath = "/api/v1/auth/login"

// invalidCredentialsMessage es el texto exacto del 401 que fija el contrato, idéntico para correo
// inexistente y contraseña incorrecta (FR-014, RN-06).
const invalidCredentialsMessage = "Correo electrónico o contraseña incorrectos."

// sessionLifetime es el plazo fijo de una sesión desde su emisión (FR-018).
const sessionLifetime = 8 * time.Hour

// Datos de la cuenta de los escenarios de US2.
const (
	anaFullName = "Ana Pérez"
	anaEmail    = "ana@example.com"
	anaPassword = "Proyecto2026"
)

// loginResponseBody refleja LoginResponse del contrato; se declara en la prueba para no depender de
// los DTO de producción al leer la respuesta.
type loginResponseBody struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// seedAccount inserta directamente en users la cuenta de Ana con su contraseña hasheada, para que
// las pruebas de US2 no dependan del endpoint de registro.
func seedAccount(t *testing.T, db *gorm.DB) domain.User {
	t.Helper()
	hash, err := service.HashPassword(anaPassword)
	if err != nil {
		t.Fatalf("no se pudo hashear la contraseña de la cuenta: %v", err)
	}
	user := domain.User{ID: uuid.New(), FullName: anaFullName, Email: anaEmail, PasswordHash: hash}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("no se pudo preparar la cuenta: %v", err)
	}
	return user
}

// postLogin envía el correo y la contraseña a la ruta de inicio de sesión y devuelve la respuesta.
func postLogin(t *testing.T, router *gin.Engine, email, password string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"email": email, "password": password})
	if err != nil {
		t.Fatalf("no se pudo serializar la solicitud: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, loginPath, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// requireIssuedSession exige un 200 con LoginResponse (solo token y expiresAt, sin la contraseña),
// un JWT firmado con el secreto de test cuyo sub es la cuenta y cuyo jti es una fila vigente de
// sessions, y un vencimiento de 8 horas fijas desde la emisión, entre before y after (FR-013,
// FR-018). Devuelve la respuesta decodificada y la fila de la sesión.
func requireIssuedSession(t *testing.T, db *gorm.DB, secret string, rec *httptest.ResponseRecorder, userID uuid.UUID, before, after time.Time) (loginResponseBody, domain.Session) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("estado %d, se esperaba %d; cuerpo: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), anaPassword) {
		t.Errorf("la respuesta contiene la contraseña en claro: %s", rec.Body.String())
	}

	// El cuerpo se lee primero como mapa para detectar campos de más y después como LoginResponse.
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("el cuerpo del 200 no es JSON válido: %v; cuerpo: %s", err, rec.Body.String())
	}
	for key := range raw {
		if key != "token" && key != "expiresAt" {
			t.Errorf("la respuesta tiene el campo inesperado %q (solo token y expiresAt)", key)
		}
	}
	var resp loginResponseBody
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("el cuerpo del 200 no respeta LoginResponse: %v; cuerpo: %s", err, rec.Body.String())
	}
	if resp.Token == "" {
		t.Fatalf("la respuesta no trae token")
	}

	// El vencimiento informado está a 8 horas del momento de la solicitud; se tolera un segundo
	// por la precisión de la base y del JWT.
	if resp.ExpiresAt.Before(before.Add(sessionLifetime-time.Second)) || resp.ExpiresAt.After(after.Add(sessionLifetime+time.Second)) {
		t.Errorf("expiresAt = %s, se esperaba 8 horas después de la emisión (entre %s y %s)",
			resp.ExpiresAt, before.Add(sessionLifetime), after.Add(sessionLifetime))
	}

	// El token es válido con el secreto de test y apunta a la cuenta y a una fila de sessions.
	claims, err := service.ParseToken(secret, resp.Token, time.Now())
	if err != nil {
		t.Fatalf("el token emitido no es válido: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("sub del token = %s, se esperaba %s", claims.UserID, userID)
	}
	var session domain.Session
	if err := db.First(&session, "id = ?", claims.SessionID).Error; err != nil {
		t.Fatalf("no existe la fila de sessions del jti %s: %v", claims.SessionID, err)
	}

	// La fila es vigente, pertenece a la cuenta, vence a 8 horas fijas de su emisión y coincide con
	// el vencimiento de la respuesta y del token.
	if session.UserID != userID {
		t.Errorf("la sesión pertenece a %s, se esperaba %s", session.UserID, userID)
	}
	if session.RevokedAt != nil {
		t.Errorf("la sesión recién emitida está cerrada (revoked_at = %s)", session.RevokedAt)
	}
	if got := session.ExpiresAt.Sub(session.CreatedAt); got != sessionLifetime {
		t.Errorf("expires_at - created_at = %s, se esperaba %s", got, sessionLifetime)
	}
	if diff := resp.ExpiresAt.Sub(session.ExpiresAt).Abs(); diff >= time.Second {
		t.Errorf("expiresAt de la respuesta (%s) difiere de expires_at de la sesión (%s)", resp.ExpiresAt, session.ExpiresAt)
	}
	if diff := claims.ExpiresAt.Sub(session.ExpiresAt).Abs(); diff >= time.Second {
		t.Errorf("exp del token (%s) difiere de expires_at de la sesión (%s)", claims.ExpiresAt, session.ExpiresAt)
	}
	return resp, session
}

// requireLoginSucceededEvents exige exactamente want eventos login_succeeded, todos atribuidos a la
// cuenta, con su momento y sin correo intentado, y ningún login_failed (FR-028, FR-029, SC-009).
func requireLoginSucceededEvents(t *testing.T, db *gorm.DB, userID uuid.UUID, want int) {
	t.Helper()
	var events []domain.AuthEvent
	if err := db.Where("event_type = ?", domain.EventLoginSucceeded).Find(&events).Error; err != nil {
		t.Fatalf("no se pudo consultar auth_events: %v", err)
	}
	if len(events) != want {
		t.Fatalf("hay %d eventos login_succeeded, se esperaban %d", len(events), want)
	}
	for _, e := range events {
		if e.UserID == nil || *e.UserID != userID {
			t.Errorf("login_succeeded tiene user_id %v, se esperaba %s", e.UserID, userID)
		}
		if e.AttemptedEmail != nil {
			t.Errorf("login_succeeded tiene attempted_email %q, se esperaba nulo", *e.AttemptedEmail)
		}
		if e.CreatedAt.IsZero() {
			t.Errorf("login_succeeded no registra el momento del evento")
		}
	}
	if n := countRows(t, db, &domain.AuthEvent{}, "event_type = ?", domain.EventLoginFailed); n != 0 {
		t.Errorf("hay %d eventos login_failed, se esperaba 0", n)
	}
}

// requireInvalidCredentials exige un 401 con el mensaje genérico del contrato, sin token ni la
// contraseña enviada en el cuerpo (FR-011, FR-014).
func requireInvalidCredentials(t *testing.T, rec *httptest.ResponseRecorder, password string) {
	t.Helper()
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("estado %d, se esperaba %d; cuerpo: %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), password) {
		t.Errorf("la respuesta de error repite la contraseña: %s", rec.Body.String())
	}
	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("el cuerpo del 401 no es JSON válido: %v; cuerpo: %s", err, rec.Body.String())
	}
	if resp.Error != invalidCredentialsMessage {
		t.Errorf("error = %q, se esperaba %q", resp.Error, invalidCredentialsMessage)
	}
	if strings.Contains(rec.Body.String(), `"token"`) {
		t.Errorf("el rechazo incluye un token: %s", rec.Body.String())
	}
}

// requireLoginFailedEvent exige exactamente un evento login_failed con el correo normalizado
// intentado y el user_id esperado (nil cuando el correo no existe), y que el intento no haya creado
// sesiones ni eventos login_succeeded (FR-029, SC-009).
func requireLoginFailedEvent(t *testing.T, db *gorm.DB, wantUserID *uuid.UUID, wantEmail string) {
	t.Helper()
	var events []domain.AuthEvent
	if err := db.Where("event_type = ?", domain.EventLoginFailed).Find(&events).Error; err != nil {
		t.Fatalf("no se pudo consultar auth_events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("hay %d eventos login_failed, se esperaba exactamente 1", len(events))
	}
	event := events[0]
	switch {
	case wantUserID == nil && event.UserID != nil:
		t.Errorf("login_failed tiene user_id %s, se esperaba nulo porque el correo no existe", *event.UserID)
	case wantUserID != nil && (event.UserID == nil || *event.UserID != *wantUserID):
		t.Errorf("login_failed tiene user_id %v, se esperaba %s", event.UserID, *wantUserID)
	}
	if event.AttemptedEmail == nil || *event.AttemptedEmail != wantEmail {
		t.Errorf("login_failed tiene attempted_email %v, se esperaba %q", event.AttemptedEmail, wantEmail)
	}
	if event.CreatedAt.IsZero() {
		t.Errorf("login_failed no registra el momento del evento")
	}
	if n := countRows(t, db, &domain.Session{}, ""); n != 0 {
		t.Errorf("el intento fallido creó %d sesiones, se esperaba 0", n)
	}
	if n := countRows(t, db, &domain.AuthEvent{}, "event_type = ?", domain.EventLoginSucceeded); n != 0 {
		t.Errorf("el intento fallido dejó %d eventos login_succeeded, se esperaba 0", n)
	}
}

// Escenario: Inicio exitoso (001/US2)
// Dada la cuenta ana@example.com con Proyecto2026, iniciar sesión con esas credenciales responde 200
// con un token válido y expiresAt a 8 horas, crea una sesión vigente y deja el evento
// login_succeeded atribuido a la cuenta (FR-012, FR-013, FR-018, FR-029).
func TestUS2_InicioExitoso(t *testing.T) {
	router, db, secret := newTestRouterWithSecret(t)
	user := seedAccount(t, db)

	before := time.Now()
	rec := postLogin(t, router, anaEmail, anaPassword)
	after := time.Now()

	requireIssuedSession(t, db, secret, rec, user.ID, before, after)
	if n := countRows(t, db, &domain.Session{}, ""); n != 1 {
		t.Errorf("hay %d sesiones, se esperaba 1", n)
	}
	requireLoginSucceededEvents(t, db, user.ID, 1)
}

// Escenario: Correo con espacios y mayúsculas (001/US2)
// Iniciar sesión escribiendo "  ANA@Example.com  " es exitoso porque el correo se normaliza igual
// que en el registro (FR-012, FR-004).
func TestUS2_CorreoConEspaciosYMayusculas(t *testing.T) {
	router, db, secret := newTestRouterWithSecret(t)
	user := seedAccount(t, db)

	before := time.Now()
	rec := postLogin(t, router, "  ANA@Example.com  ", anaPassword)
	after := time.Now()

	requireIssuedSession(t, db, secret, rec, user.ID, before, after)
	requireLoginSucceededEvents(t, db, user.ID, 1)
}

// Escenario: Sesiones simultáneas (001/US2)
// Dos inicios de sesión seguidos de la misma cuenta, como desde dos dispositivos, dejan dos
// sesiones vigentes y distintas: la segunda no cierra ni invalida la primera (FR-017).
func TestUS2_SesionesSimultaneas(t *testing.T) {
	router, db, secret := newTestRouterWithSecret(t)
	user := seedAccount(t, db)

	before := time.Now()
	first := postLogin(t, router, anaEmail, anaPassword)
	second := postLogin(t, router, anaEmail, anaPassword)
	after := time.Now()

	firstResp, firstSession := requireIssuedSession(t, db, secret, first, user.ID, before, after)
	secondResp, secondSession := requireIssuedSession(t, db, secret, second, user.ID, before, after)
	if firstSession.ID == secondSession.ID || firstResp.Token == secondResp.Token {
		t.Fatalf("los dos inicios de sesión comparten sesión (%s) o token", firstSession.ID)
	}

	// Tras el segundo inicio, la primera sesión sigue vigente en la base y su token sigue válido.
	var reloaded domain.Session
	if err := db.First(&reloaded, "id = ?", firstSession.ID).Error; err != nil {
		t.Fatalf("la primera sesión desapareció: %v", err)
	}
	if reloaded.RevokedAt != nil || !reloaded.ExpiresAt.Equal(firstSession.ExpiresAt) {
		t.Errorf("la segunda sesión alteró la primera: revoked_at = %v, expires_at = %s (antes %s)",
			reloaded.RevokedAt, reloaded.ExpiresAt, firstSession.ExpiresAt)
	}
	if _, err := service.ParseToken(secret, firstResp.Token, time.Now()); err != nil {
		t.Errorf("el token de la primera sesión dejó de ser válido: %v", err)
	}
	if n := countRows(t, db, &domain.Session{}, "user_id = ? AND revoked_at IS NULL", user.ID); n != 2 {
		t.Errorf("hay %d sesiones vigentes de la cuenta, se esperaban 2", n)
	}
	requireLoginSucceededEvents(t, db, user.ID, 2)
}

// Escenario: Correo inexistente (001/US2)
// Sin cuenta para nadie@example.com, el intento con cualquier contraseña responde 401 con el
// mensaje genérico y deja login_failed con el correo normalizado y user_id nulo, sin inventar una
// cuenta (FR-014, FR-029, SC-009). La variante con espacios y mayúsculas verifica la normalización
// del correo registrado en el evento.
func TestUS2_CorreoInexistente(t *testing.T) {
	cases := []struct {
		name  string
		email string
	}{
		{"correo tal cual", "nadie@example.com"},
		{"correo con espacios y mayúsculas", "  Nadie@Example.COM  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, db := newTestRouter(t)
			seedAccount(t, db)

			rec := postLogin(t, router, tc.email, "CualquierCosa1")

			requireInvalidCredentials(t, rec, "CualquierCosa1")
			requireLoginFailedEvent(t, db, nil, "nadie@example.com")
			if n := countRows(t, db, &domain.User{}, ""); n != 1 {
				t.Errorf("hay %d cuentas, se esperaba solo la existente", n)
			}
		})
	}
}

// Escenario: Contraseña incorrecta (001/US2)
// Con la cuenta ana@example.com, una contraseña incorrecta responde 401 con el mismo mensaje del
// correo inexistente y deja login_failed atribuido a la cuenta. Las respuestas de ambos fallos se
// comparan byte a byte (estado, Content-Type y cuerpo) para que no revelen qué correos existen
// (FR-014, RN-06, SC-004).
func TestUS2_ContrasenaIncorrecta(t *testing.T) {
	router, db := newTestRouter(t)
	user := seedAccount(t, db)

	wrongPassword := postLogin(t, router, anaEmail, "Incorrecta123")

	requireInvalidCredentials(t, wrongPassword, "Incorrecta123")
	requireLoginFailedEvent(t, db, &user.ID, anaEmail)

	// Se compara con la respuesta al correo inexistente, obtenida con la misma contraseña.
	unknownEmail := postLogin(t, router, "nadie@example.com", "Incorrecta123")
	if wrongPassword.Code != unknownEmail.Code {
		t.Errorf("estados distintos: contraseña incorrecta %d, correo inexistente %d", wrongPassword.Code, unknownEmail.Code)
	}
	if a, b := wrongPassword.Header().Get("Content-Type"), unknownEmail.Header().Get("Content-Type"); a != b {
		t.Errorf("Content-Type distinto: contraseña incorrecta %q, correo inexistente %q", a, b)
	}
	if !bytes.Equal(wrongPassword.Body.Bytes(), unknownEmail.Body.Bytes()) {
		t.Errorf("cuerpos distintos:\ncontraseña incorrecta: %q\ncorreo inexistente:    %q",
			wrongPassword.Body.Bytes(), unknownEmail.Body.Bytes())
	}
}

// Pruebas de integración de las ramas de entrada y de error de POST /api/v1/auth/login (T086). No
// son escenarios Given/When/Then de US2, así que se nombran por el manejador y no por la historia.

// Mensajes exactos del contrato para el cuerpo ilegible y los campos faltantes, y mensaje genérico
// del 500, que no expone detalles internos (FR-011).
const (
	invalidBodyMessage   = "El cuerpo de la solicitud debe ser un JSON válido."
	invalidFieldsMessage = "Hay campos inválidos en la solicitud."
	internalErrorMessage = "Ocurrió un error inesperado. Intentá de nuevo más tarde."
)

// postLoginRaw envía a la ruta de inicio de sesión el cuerpo tal cual, sin serializarlo.
func postLoginRaw(t *testing.T, router *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, loginPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// requireErrorResponse exige el estado indicado con un ErrorResponse cuyo único campo es error con
// el mensaje esperado, sin token ni detalle de campos.
func requireErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantMessage string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("estado %d, se esperaba %d; cuerpo: %s", rec.Code, wantStatus, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("el cuerpo no es JSON válido: %v; cuerpo: %s", err, rec.Body.String())
	}
	if len(resp) != 1 || resp["error"] != wantMessage {
		t.Errorf("cuerpo %s, se esperaba solo {\"error\": %q}", rec.Body.String(), wantMessage)
	}
}

// requireNoLoginSideEffects exige que el intento no haya creado sesiones ni registrado eventos.
func requireNoLoginSideEffects(t *testing.T, db *gorm.DB) {
	t.Helper()
	if n := countRows(t, db, &domain.Session{}, ""); n != 0 {
		t.Errorf("se crearon %d sesiones, se esperaba 0", n)
	}
	if n := countRows(t, db, &domain.AuthEvent{}, ""); n != 0 {
		t.Errorf("se registraron %d eventos, se esperaba 0", n)
	}
}

// Un cuerpo que no es un JSON válido, o que falta, responde 400 con ErrorResponse y el mensaje del
// contrato, nunca 500 ni ValidationErrorResponse, y no deja sesiones ni eventos (Principio VII).
func TestLoginUser_InvalidJSONBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"JSON mal formado", `{"email": "ana@example.com", "password": `},
		{"texto que no es JSON", "email=ana@example.com&password=Proyecto2026"},
		{"cuerpo vacío", ""},
		{"campo con tipo incorrecto", `{"email": 42, "password": "Proyecto2026"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, db := newTestRouter(t)
			seedAccount(t, db)

			rec := postLoginRaw(t, router, tc.body)

			requireErrorResponse(t, rec, http.StatusBadRequest, invalidBodyMessage)
			requireNoLoginSideEffects(t, db)
		})
	}
}

// Un correo o una contraseña vacíos responden 400 con ValidationErrorResponse que señala
// exactamente los campos faltantes, sin llegar al servicio: no se registra login_failed ni se crea
// sesión. El correo de solo espacios cuenta como vacío porque se valida ya normalizado.
func TestLoginUser_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
		want []string
	}{
		{"correo vacío", map[string]any{"email": "", "password": anaPassword}, []string{"email"}},
		{"correo de solo espacios", map[string]any{"email": "   ", "password": anaPassword}, []string{"email"}},
		{"correo ausente", map[string]any{"password": anaPassword}, []string{"email"}},
		{"contraseña vacía", map[string]any{"email": anaEmail, "password": ""}, []string{"password"}},
		{"contraseña ausente", map[string]any{"email": anaEmail}, []string{"password"}},
		{"correo y contraseña vacíos", map[string]any{"email": "", "password": ""}, []string{"email", "password"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, db := newTestRouter(t)
			seedAccount(t, db)
			payload, err := json.Marshal(tc.body)
			if err != nil {
				t.Fatalf("no se pudo serializar la solicitud: %v", err)
			}

			rec := postLoginRaw(t, router, string(payload))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("estado %d, se esperaba %d; cuerpo: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			var resp validationErrorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("el cuerpo del 400 no es JSON válido: %v; cuerpo: %s", err, rec.Body.String())
			}
			if resp.Error != invalidFieldsMessage {
				t.Errorf("error = %q, se esperaba %q", resp.Error, invalidFieldsMessage)
			}
			got := make([]string, 0, len(resp.Fields))
			for _, f := range resp.Fields {
				got = append(got, f.Field)
				if f.Message == "" {
					t.Errorf("el campo %q no trae mensaje", f.Field)
				}
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("campos señalados %v, se esperaba %v", got, tc.want)
			}
			requireNoLoginSideEffects(t, db)
		})
	}
}

// Si la base falla durante el inicio de sesión, la respuesta es 500 con el mensaje genérico y sin
// filtrar el error interno de la base ni la contraseña (Principio VII, FR-011). El fallo se provoca
// cerrando el pool de conexiones del router de test antes de la solicitud.
func TestLoginUser_DatabaseFailureReturnsGenericError(t *testing.T) {
	router, db := newTestRouter(t)
	seedAccount(t, db)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("no se pudo obtener el pool de conexiones: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("no se pudo cerrar el pool de conexiones: %v", err)
	}

	rec := postLogin(t, router, anaEmail, anaPassword)

	requireErrorResponse(t, rec, http.StatusInternalServerError, internalErrorMessage)
	body := strings.ToLower(rec.Body.String())
	for _, leak := range []string{"sql", "closed", "database", "buscar cuenta", anaEmail, strings.ToLower(anaPassword)} {
		if strings.Contains(body, leak) {
			t.Errorf("el 500 filtra %q: %s", leak, rec.Body.String())
		}
	}
}

// Pruebas de integración de US4 (T062): cubren GET /api/v1/users/me de punta a punta, con la sesión
// emitida por el inicio de sesión real y verificada por el middleware contra la base smye_test. El
// endpoint no acepta ningún identificador: el perfil se resuelve solo desde la sesión (FR-022,
// FR-023).

// profilePath es la ruta protegida del perfil propio según contracts/openapi.yaml.
const profilePath = "/api/v1/users/me"

// Datos de una segunda cuenta, para comprobar que una sesión nunca devuelve el perfil de otra.
const (
	brunoFullName = "Bruno Díaz"
	brunoEmail    = "bruno@example.com"
	brunoPassword = "Otraclave2026"
)

// seedNamedAccount inserta directamente en users una cuenta con los datos indicados y su contraseña
// hasheada, sin pasar por el endpoint de registro.
func seedNamedAccount(t *testing.T, db *gorm.DB, fullName, email, password string) domain.User {
	t.Helper()
	hash, err := service.HashPassword(password)
	if err != nil {
		t.Fatalf("no se pudo hashear la contraseña de la cuenta: %v", err)
	}
	user := domain.User{ID: uuid.New(), FullName: fullName, Email: email, PasswordHash: hash}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("no se pudo preparar la cuenta %s: %v", email, err)
	}
	return user
}

// loginTokenFor inicia sesión con las credenciales indicadas y devuelve el token emitido.
func loginTokenFor(t *testing.T, router *gin.Engine, email, password string) string {
	t.Helper()
	rec := postLogin(t, router, email, password)
	if rec.Code != http.StatusOK {
		t.Fatalf("el inicio de sesión de preparación de %s devolvió %d; cuerpo: %s", email, rec.Code, rec.Body.String())
	}
	var body loginResponseBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Token == "" {
		t.Fatalf("el inicio de sesión de preparación no devolvió un token: %v; cuerpo: %s", err, rec.Body.String())
	}
	return body.Token
}

// getProfile pide el perfil a la ruta indicada con la cabecera Authorization recibida; si es vacía,
// la petición sale sin cabecera, como la de un visitante sin sesión.
func getProfile(t *testing.T, router *gin.Engine, target, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// requireOwnProfile exige un 200 cuyo cuerpo sea exactamente el UserProfile de la cuenta indicada:
// solo id, fullName y email, con los valores guardados de esa cuenta (FR-022).
func requireOwnProfile(t *testing.T, rec *httptest.ResponseRecorder, want domain.User) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("estado %d, se esperaba %d; cuerpo: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var profile map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &profile); err != nil {
		t.Fatalf("el cuerpo del 200 no es JSON válido: %v; cuerpo: %s", err, rec.Body.String())
	}
	if len(profile) != 3 {
		t.Errorf("el perfil tiene %d campos, se esperaban solo id, fullName y email; cuerpo: %s", len(profile), rec.Body.String())
	}
	wantFields := map[string]string{"id": want.ID.String(), "fullName": want.FullName, "email": want.Email}
	for key, value := range wantFields {
		if profile[key] != value {
			t.Errorf("%s = %v, se esperaba %q", key, profile[key], value)
		}
	}
}

// Escenario: Perfil propio (001/US4)
// Dado un usuario con sesión vigente, cuando consulta su perfil, entonces ve su nombre completo y su
// correo. Con dos cuentas en el sistema, cada sesión devuelve solo el perfil de su titular, y ningún
// identificador enviado en la consulta ni en la ruta permite ver el perfil de otra cuenta (FR-022,
// FR-023).
func TestUS4_PerfilPropio(t *testing.T) {
	router, db := newTestRouter(t)
	ana := seedAccount(t, db)
	bruno := seedNamedAccount(t, db, brunoFullName, brunoEmail, brunoPassword)
	anaToken := loginTokenFor(t, router, anaEmail, anaPassword)
	brunoToken := loginTokenFor(t, router, brunoEmail, brunoPassword)

	t.Run("cada titular ve su propio perfil", func(t *testing.T) {
		requireOwnProfile(t, getProfile(t, router, profilePath, "Bearer "+anaToken), ana)
		requireOwnProfile(t, getProfile(t, router, profilePath, "Bearer "+brunoToken), bruno)
	})

	t.Run("un identificador en la consulta no cambia el titular", func(t *testing.T) {
		for _, param := range []string{"id", "userId", "user_id", "email"} {
			value := bruno.ID.String()
			if param == "email" {
				value = bruno.Email
			}
			target := profilePath + "?" + param + "=" + value
			requireOwnProfile(t, getProfile(t, router, target, "Bearer "+anaToken), ana)
		}
	})

	t.Run("no existe ruta de perfil por identificador", func(t *testing.T) {
		rec := getProfile(t, router, "/api/v1/users/"+bruno.ID.String(), "Bearer "+anaToken)
		if rec.Code == http.StatusOK {
			t.Errorf("GET /api/v1/users/{id} respondió 200; no debe existir forma de consultar otra cuenta (FR-023); cuerpo: %s", rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), brunoEmail) || strings.Contains(rec.Body.String(), brunoFullName) {
			t.Errorf("la respuesta revela datos de otra cuenta: %s", rec.Body.String())
		}
	})
}

// Escenario: Correo normalizado en perfil (001/US4)
// Dado un usuario que se registró con "  Ana@Example.COM  ", cuando consulta su perfil, entonces ve
// el correo tal como quedó normalizado, el mismo que usa para iniciar sesión (FR-004).
func TestUS4_CorreoNormalizadoEnPerfil(t *testing.T) {
	router, db := newTestRouter(t)
	body := validRegistration()
	body["email"] = "  Ana@Example.COM  "
	id := requireCreatedProfile(t, postRegister(t, router, body), anaPassword, anaFullName, anaEmail)
	token := loginTokenFor(t, router, "  ANA@example.com ", anaPassword)

	rec := getProfile(t, router, profilePath, "Bearer "+token)

	var stored domain.User
	if err := db.First(&stored, "id = ?", id).Error; err != nil {
		t.Fatalf("no se encontró la cuenta registrada: %v", err)
	}
	if stored.Email != anaEmail {
		t.Fatalf("correo guardado %q, se esperaba %q", stored.Email, anaEmail)
	}
	requireOwnProfile(t, rec, stored)

	// El correo que muestra el perfil sirve tal cual para iniciar sesión.
	if again := postLogin(t, router, stored.Email, anaPassword); again.Code != http.StatusOK {
		t.Errorf("iniciar sesión con el correo del perfil devolvió %d; cuerpo: %s", again.Code, again.Body.String())
	}
}

// Escenario: Respuesta sin contraseña (001/US4)
// Dado un usuario cuyo perfil se consulta, cuando se inspecciona la respuesta completa (cuerpo y
// cabeceras), entonces no aparece la contraseña, ni el hash guardado, ni ninguna representación
// derivada, ni campos con nombre de contraseña o hash (FR-011, SC-005).
func TestUS4_RespuestaSinContrasena(t *testing.T) {
	router, db := newTestRouter(t)
	ana := seedAccount(t, db)
	token := loginTokenFor(t, router, anaEmail, anaPassword)

	rec := getProfile(t, router, profilePath, "Bearer "+token)

	requireOwnProfile(t, rec, ana)

	// Representaciones prohibidas: la contraseña en claro y codificada, el hash completo, su parte de
	// sal y digesto sin el prefijo de versión y coste, y los prefijos bcrypt, también escapados.
	forbidden := map[string]string{
		"contraseña en claro":      anaPassword,
		"contraseña en minúsculas": strings.ToLower(anaPassword),
		"contraseña en base64":     base64.StdEncoding.EncodeToString([]byte(anaPassword)),
		"contraseña en base64url":  base64.RawURLEncoding.EncodeToString([]byte(anaPassword)),
		"hash bcrypt completo":     ana.PasswordHash,
		"sal y digesto del hash":   ana.PasswordHash[len(ana.PasswordHash)-53:],
		"prefijo bcrypt $2a$":      "$2a$",
		"prefijo bcrypt $2b$":      "$2b$",
		"prefijo bcrypt escapado":  `$2`,
	}
	var headers strings.Builder
	for name, values := range rec.Header() {
		headers.WriteString(name + ": " + strings.Join(values, ",") + "\n")
	}
	for label, value := range forbidden {
		if strings.Contains(rec.Body.String(), value) {
			t.Errorf("el cuerpo contiene %s: %s", label, rec.Body.String())
		}
		if strings.Contains(headers.String(), value) {
			t.Errorf("las cabeceras contienen %s: %s", label, headers.String())
		}
	}

	// Ningún campo, en ninguna forma de escritura, se llama como la contraseña o su hash.
	lowerBody := strings.ToLower(rec.Body.String())
	for _, name := range []string{"password", "passwordhash", "password_hash", "hash", "contraseña", "contrasena"} {
		if strings.Contains(lowerBody, name) {
			t.Errorf("el cuerpo menciona %q: %s", name, rec.Body.String())
		}
	}
}

// Escenario: Perfil sin sesión (001/US4)
// Dado un visitante sin sesión válida, cuando consulta el perfil, entonces el acceso se rechaza con
// el 401 del contrato, idéntico en todos los casos, sin revelar ningún dato de ninguna cuenta
// (FR-015, FR-016, SC-003).
func TestUS4_PerfilSinSesion(t *testing.T) {
	router, db, secret := newTestRouterWithSecret(t)
	ana := seedAccount(t, db)
	token := loginTokenFor(t, router, anaEmail, anaPassword)
	now := time.Now()

	// signedSession guarda una sesión de Ana con los momentos indicados y devuelve su token firmado
	// con el secreto de test.
	signedSession := func(createdAt, expiresAt time.Time, revokedAt *time.Time) string {
		t.Helper()
		session := domain.Session{ID: uuid.New(), UserID: ana.ID, CreatedAt: createdAt, ExpiresAt: expiresAt, RevokedAt: revokedAt}
		if err := db.Create(&session).Error; err != nil {
			t.Fatalf("no se pudo preparar la sesión: %v", err)
		}
		signed, err := service.IssueToken(secret, session)
		if err != nil {
			t.Fatalf("no se pudo firmar el token de preparación: %v", err)
		}
		return signed
	}
	revokedAt := now.Add(-time.Minute)
	expiredToken := signedSession(now.Add(-9*time.Hour), now.Add(-time.Hour), nil)
	closedToken := signedSession(now.Add(-time.Hour), now.Add(7*time.Hour), &revokedAt)

	cases := []struct {
		name          string
		authorization string
	}{
		{"sin cabecera Authorization", ""},
		{"texto que no es un JWT", "Bearer no-es-un-jwt"},
		{"firma alterada", "Bearer " + token[:len(token)-2] + flipLastChars(token[len(token)-2:])},
		{"esquema distinto de Bearer", "Basic " + token},
		{"sesión vencida", "Bearer " + expiredToken},
		{"sesión cerrada", "Bearer " + closedToken},
	}
	baseline := getProfile(t, router, profilePath, "")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := getProfile(t, router, profilePath, tc.authorization)

			requireErrorResponse(t, rec, http.StatusUnauthorized, sessionRequiredMessage)
			if rec.Body.String() != baseline.Body.String() ||
				rec.Header().Get("Content-Type") != baseline.Header().Get("Content-Type") {
				t.Errorf("el rechazo (%q, %s) difiere del de sesión ausente (%q, %s)",
					rec.Header().Get("Content-Type"), rec.Body.String(),
					baseline.Header().Get("Content-Type"), baseline.Body.String())
			}
			for _, leak := range []string{ana.ID.String(), anaEmail, anaFullName} {
				if strings.Contains(rec.Body.String(), leak) {
					t.Errorf("el rechazo revela %q: %s", leak, rec.Body.String())
				}
			}
		})
	}
}

// Pruebas de integración de US5 (T068): cubren POST /api/v1/auth/logout de punta a punta, con la
// sesión emitida por el inicio de sesión real, verificada por el middleware y revocada en la base
// smye_test. El cierre invalida de inmediato solo la sesión de la petición, sin tocar su
// vencimiento, y deja el evento logout atribuido a la cuenta (FR-017, FR-024 a FR-026, FR-028).

// logoutPath es la ruta protegida de cierre de sesión según contracts/openapi.yaml.
const logoutPath = "/api/v1/auth/logout"

// postLogout pide el cierre de sesión con la cabecera Authorization recibida; si es vacía, la
// petición sale sin cabecera, como la de un visitante sin sesión.
func postLogout(t *testing.T, router *gin.Engine, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, logoutPath, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// requireLoggedOut exige el 204 del contrato, sin cuerpo, como confirmación del cierre (FR-024).
func requireLoggedOut(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusNoContent {
		t.Fatalf("estado %d, se esperaba %d; cuerpo: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("el 204 trae cuerpo: %s", rec.Body.String())
	}
}

// sessionOf devuelve la fila de sessions que corresponde al jti del token, leída en este momento.
func sessionOf(t *testing.T, db *gorm.DB, secret, token string) domain.Session {
	t.Helper()
	claims, err := service.ParseToken(secret, token, time.Now())
	if err != nil {
		t.Fatalf("el token de preparación no es válido: %v", err)
	}
	var session domain.Session
	if err := db.First(&session, "id = ?", claims.SessionID).Error; err != nil {
		t.Fatalf("no existe la fila de sessions del jti %s: %v", claims.SessionID, err)
	}
	return session
}

// requireLogoutEvents exige exactamente want eventos logout, todos atribuidos a la cuenta, con su
// momento y sin correo intentado (FR-028, FR-029, SC-009).
func requireLogoutEvents(t *testing.T, db *gorm.DB, userID uuid.UUID, want int) {
	t.Helper()
	var events []domain.AuthEvent
	if err := db.Where("event_type = ?", domain.EventLogout).Find(&events).Error; err != nil {
		t.Fatalf("no se pudo consultar auth_events: %v", err)
	}
	if len(events) != want {
		t.Fatalf("hay %d eventos logout, se esperaban %d", len(events), want)
	}
	for _, e := range events {
		if e.UserID == nil || *e.UserID != userID {
			t.Errorf("logout tiene user_id %v, se esperaba %s", e.UserID, userID)
		}
		if e.AttemptedEmail != nil {
			t.Errorf("logout tiene attempted_email %q, se esperaba nulo", *e.AttemptedEmail)
		}
		if e.CreatedAt.IsZero() {
			t.Errorf("logout no registra el momento del evento")
		}
	}
}

// Escenario: Cierre exitoso (001/US5)
// Dado un usuario con sesión vigente, cuando cierra su sesión, entonces el sistema confirma el
// cierre con 204, marca revoked_at en esa sesión sin tocar su vencimiento y deja el evento logout
// atribuido a la cuenta (FR-018, FR-024, FR-026, FR-028, SC-009).
func TestUS5_CierreExitoso(t *testing.T) {
	router, db, secret := newTestRouterWithSecret(t)
	ana := seedAccount(t, db)
	token := loginTokenFor(t, router, anaEmail, anaPassword)
	issued := sessionOf(t, db, secret, token)

	before := time.Now()
	rec := postLogout(t, router, "Bearer "+token)
	after := time.Now()

	requireLoggedOut(t, rec)
	closed := sessionOf(t, db, secret, token)
	if closed.RevokedAt == nil {
		t.Fatalf("la sesión sigue sin revoked_at después del cierre")
	}
	if closed.RevokedAt.Before(before.Add(-time.Second)) || closed.RevokedAt.After(after.Add(time.Second)) {
		t.Errorf("revoked_at = %s, se esperaba el momento del cierre (entre %s y %s)", closed.RevokedAt, before, after)
	}
	if !closed.ExpiresAt.Equal(issued.ExpiresAt) {
		t.Errorf("el cierre modificó expires_at: %s (antes %s)", closed.ExpiresAt, issued.ExpiresAt)
	}
	requireLogoutEvents(t, db, ana.ID, 1)
}

// Escenario: Nuevo inicio tras cierre (001/US5)
// Dado un usuario que cerró su sesión, cuando vuelve a iniciar sesión con credenciales correctas,
// entonces obtiene una sesión nueva y vigente: una fila nueva en sessions, distinta de la cerrada,
// que sigue cerrada (FR-025).
func TestUS5_NuevoInicioTrasCierre(t *testing.T) {
	router, db, secret := newTestRouterWithSecret(t)
	ana := seedAccount(t, db)
	oldToken := loginTokenFor(t, router, anaEmail, anaPassword)
	oldSession := sessionOf(t, db, secret, oldToken)
	requireLoggedOut(t, postLogout(t, router, "Bearer "+oldToken))

	before := time.Now()
	rec := postLogin(t, router, anaEmail, anaPassword)
	after := time.Now()

	resp, newSession := requireIssuedSession(t, db, secret, rec, ana.ID, before, after)
	if newSession.ID == oldSession.ID || resp.Token == oldToken {
		t.Fatalf("el nuevo inicio reutilizó la sesión cerrada (%s) o su token", oldSession.ID)
	}
	if n := countRows(t, db, &domain.Session{}, "user_id = ?", ana.ID); n != 2 {
		t.Errorf("hay %d filas en sessions, se esperaban 2 (la cerrada y la nueva)", n)
	}
	if reloaded := sessionOf(t, db, secret, oldToken); reloaded.RevokedAt == nil {
		t.Errorf("el nuevo inicio reabrió la sesión cerrada %s", oldSession.ID)
	}
	requireOwnProfile(t, getProfile(t, router, profilePath, "Bearer "+resp.Token), ana)
	requireLoginSucceededEvents(t, db, ana.ID, 2)
	requireLogoutEvents(t, db, ana.ID, 1)
}

// Escenario: Token cerrado no sirve (001/US5)
// Dado un usuario que cerró su sesión, cuando se intenta usar esa misma sesión para acceder a una
// función protegida, desde su dispositivo o desde cualquier otro, entonces el acceso se rechaza con
// el 401 del contrato aunque no hayan pasado las 8 horas de vigencia (FR-025, FR-026). Volver a
// cerrar con el mismo token tampoco tiene efecto.
func TestUS5_TokenCerradoNoSirve(t *testing.T) {
	router, db, secret := newTestRouterWithSecret(t)
	ana := seedAccount(t, db)
	token := loginTokenFor(t, router, anaEmail, anaPassword)
	requireLoggedOut(t, postLogout(t, router, "Bearer "+token))

	// La sesión cerrada todavía tendría horas de vigencia: el rechazo se debe solo al cierre.
	closed := sessionOf(t, db, secret, token)
	if !closed.ExpiresAt.After(time.Now().Add(sessionLifetime - time.Minute)) {
		t.Fatalf("expires_at = %s ya no está a casi 8 horas; la prueba no aislaría el cierre", closed.ExpiresAt)
	}

	t.Run("desde el mismo dispositivo", func(t *testing.T) {
		rec := getProfile(t, router, profilePath, "Bearer "+token)
		requireErrorResponse(t, rec, http.StatusUnauthorized, sessionRequiredMessage)
	})

	t.Run("copiado en otro dispositivo", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, profilePath, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("User-Agent", "otro-dispositivo/1.0")
		req.RemoteAddr = "203.0.113.7:51000"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		requireErrorResponse(t, rec, http.StatusUnauthorized, sessionRequiredMessage)
		if strings.Contains(rec.Body.String(), anaEmail) {
			t.Errorf("el rechazo revela datos de la cuenta: %s", rec.Body.String())
		}
	})

	t.Run("un segundo cierre con el mismo token", func(t *testing.T) {
		rec := postLogout(t, router, "Bearer "+token)
		requireErrorResponse(t, rec, http.StatusUnauthorized, sessionRequiredMessage)
		if again := sessionOf(t, db, secret, token); again.RevokedAt == nil || !again.RevokedAt.Equal(*closed.RevokedAt) {
			t.Errorf("el segundo cierre alteró revoked_at: %v (antes %v)", again.RevokedAt, closed.RevokedAt)
		}
	})

	requireLogoutEvents(t, db, ana.ID, 1)
}

// Escenario: Cierre no afecta otras sesiones (001/US5)
// Dado un usuario con sesiones vigentes en dos dispositivos, cuando cierra la sesión en uno,
// entonces la del otro dispositivo sigue funcionando con normalidad (FR-017, FR-026).
func TestUS5_CierreNoAfectaOtrasSesiones(t *testing.T) {
	router, db, secret := newTestRouterWithSecret(t)
	ana := seedAccount(t, db)
	laptopToken := loginTokenFor(t, router, anaEmail, anaPassword)
	phoneToken := loginTokenFor(t, router, anaEmail, anaPassword)
	phoneBefore := sessionOf(t, db, secret, phoneToken)

	requireLoggedOut(t, postLogout(t, router, "Bearer "+laptopToken))

	// La sesión del otro dispositivo sigue intacta en la base y sigue dando acceso.
	phoneAfter := sessionOf(t, db, secret, phoneToken)
	if phoneAfter.RevokedAt != nil || !phoneAfter.ExpiresAt.Equal(phoneBefore.ExpiresAt) {
		t.Errorf("el cierre alteró la otra sesión: revoked_at = %v, expires_at = %s (antes %s)",
			phoneAfter.RevokedAt, phoneAfter.ExpiresAt, phoneBefore.ExpiresAt)
	}
	requireOwnProfile(t, getProfile(t, router, profilePath, "Bearer "+phoneToken), ana)

	// Solo la sesión cerrada deja de servir y solo ella queda revocada.
	if laptop := sessionOf(t, db, secret, laptopToken); laptop.RevokedAt == nil {
		t.Errorf("la sesión cerrada sigue sin revoked_at")
	}
	requireErrorResponse(t, getProfile(t, router, profilePath, "Bearer "+laptopToken), http.StatusUnauthorized, sessionRequiredMessage)
	if n := countRows(t, db, &domain.Session{}, "user_id = ? AND revoked_at IS NULL", ana.ID); n != 1 {
		t.Errorf("hay %d sesiones vigentes de la cuenta, se esperaba 1", n)
	}
	requireLogoutEvents(t, db, ana.ID, 1)
}

// Escenario: Cierre sin sesión (001/US5)
// Dado un visitante sin sesión válida, cuando solicita cerrar sesión, entonces recibe el 401 del
// contrato, idéntico en todos los casos, la operación no produce ningún efecto (ninguna sesión
// cambia y no se registra logout) y no revela información de ninguna cuenta (FR-015, FR-016).
func TestUS5_CierreSinSesion(t *testing.T) {
	router, db, secret := newTestRouterWithSecret(t)
	ana := seedAccount(t, db)
	token := loginTokenFor(t, router, anaEmail, anaPassword)
	now := time.Now()

	// signedSession guarda una sesión de Ana con los momentos indicados y devuelve su token firmado
	// con el secreto de test.
	signedSession := func(createdAt, expiresAt time.Time, revokedAt *time.Time) string {
		t.Helper()
		session := domain.Session{ID: uuid.New(), UserID: ana.ID, CreatedAt: createdAt, ExpiresAt: expiresAt, RevokedAt: revokedAt}
		if err := db.Create(&session).Error; err != nil {
			t.Fatalf("no se pudo preparar la sesión: %v", err)
		}
		signed, err := service.IssueToken(secret, session)
		if err != nil {
			t.Fatalf("no se pudo firmar el token de preparación: %v", err)
		}
		return signed
	}
	revokedAt := now.Add(-time.Minute)
	expiredToken := signedSession(now.Add(-9*time.Hour), now.Add(-time.Hour), nil)
	closedToken := signedSession(now.Add(-time.Hour), now.Add(7*time.Hour), &revokedAt)

	// snapshot lee todas las sesiones ordenadas por id para comparar el estado antes y después.
	snapshot := func() []domain.Session {
		t.Helper()
		var sessions []domain.Session
		if err := db.Order("id").Find(&sessions).Error; err != nil {
			t.Fatalf("no se pudo leer sessions: %v", err)
		}
		return sessions
	}
	before := snapshot()

	cases := []struct {
		name          string
		authorization string
	}{
		{"sin cabecera Authorization", ""},
		{"texto que no es un JWT", "Bearer no-es-un-jwt"},
		{"firma alterada", "Bearer " + token[:len(token)-2] + flipLastChars(token[len(token)-2:])},
		{"esquema distinto de Bearer", "Basic " + token},
		{"Bearer sin token", "Bearer "},
		{"sesión vencida", "Bearer " + expiredToken},
		{"sesión ya cerrada", "Bearer " + closedToken},
	}
	baseline := getProfile(t, router, profilePath, "")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postLogout(t, router, tc.authorization)

			requireErrorResponse(t, rec, http.StatusUnauthorized, sessionRequiredMessage)
			if rec.Body.String() != baseline.Body.String() ||
				rec.Header().Get("Content-Type") != baseline.Header().Get("Content-Type") {
				t.Errorf("el rechazo (%q, %s) difiere del de sesión ausente (%q, %s)",
					rec.Header().Get("Content-Type"), rec.Body.String(),
					baseline.Header().Get("Content-Type"), baseline.Body.String())
			}
			for _, leak := range []string{ana.ID.String(), anaEmail, anaFullName} {
				if strings.Contains(rec.Body.String(), leak) {
					t.Errorf("el rechazo revela %q: %s", leak, rec.Body.String())
				}
			}
		})
	}

	// Ningún intento cambió las sesiones ni registró eventos logout, y la sesión vigente de Ana
	// sigue dando acceso.
	after := snapshot()
	if len(after) != len(before) {
		t.Fatalf("hay %d sesiones, antes había %d", len(after), len(before))
	}
	for i := range before {
		b, a := before[i], after[i]
		sameRevoked := (b.RevokedAt == nil && a.RevokedAt == nil) ||
			(b.RevokedAt != nil && a.RevokedAt != nil && b.RevokedAt.Equal(*a.RevokedAt))
		if !sameRevoked || !b.ExpiresAt.Equal(a.ExpiresAt) {
			t.Errorf("la sesión %s cambió: revoked_at %v -> %v, expires_at %s -> %s",
				b.ID, b.RevokedAt, a.RevokedAt, b.ExpiresAt, a.ExpiresAt)
		}
	}
	if n := countRows(t, db, &domain.AuthEvent{}, "event_type = ?", domain.EventLogout); n != 0 {
		t.Errorf("se registraron %d eventos logout, se esperaba 0", n)
	}
	requireOwnProfile(t, getProfile(t, router, profilePath, "Bearer "+token), ana)
}

// Pruebas de integración del límite global del cuerpo de las solicitudes en las rutas públicas
// (T094, hallazgo de seguridad detectado en la US3 de la 002): un JSON de más de 1 MiB se rechaza con
// 413 o 400 antes de que el servicio lo procese. Los campos esperados van completos y válidos, y el
// exceso viaja en un campo que los DTO ignoran, de modo que sin el límite la solicitud llegaría al
// servicio y crearía la cuenta o la sesión.

// maxRequestBodyBytes es el tope del cuerpo de las solicitudes que fija el hallazgo: 1 MiB.
const maxRequestBodyBytes = 1 << 20

// oversizedPadding es el valor de relleno que, solo, ya supera el tope del cuerpo.
func oversizedPadding() string {
	return strings.Repeat("a", maxRequestBodyBytes+1)
}

// requireBodyTooLarge exige que la respuesta sea 413 o 400, los dos rechazos admitidos para un cuerpo
// que excede el tope.
func requireBodyTooLarge(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusBadRequest {
		t.Errorf("un cuerpo de más de 1 MiB debería rechazarse con 413 o 400, se obtuvo %d", rec.Code)
	}
}

// Un registro válido dentro de un cuerpo de más de 1 MiB responde 413 o 400 y no crea la cuenta.
func TestRegisterUser_OversizedBody(t *testing.T) {
	router, db := newTestRouter(t)
	body := validRegistration()
	body["padding"] = oversizedPadding()

	rec := postRegister(t, router, body)

	requireBodyTooLarge(t, rec)
	if n := countRows(t, db, &domain.User{}, ""); n != 0 {
		t.Errorf("se crearon %d cuentas, se esperaba 0", n)
	}
}

// Un inicio de sesión con credenciales correctas dentro de un cuerpo de más de 1 MiB responde 413 o
// 400, sin crear sesiones ni registrar eventos.
func TestLoginUser_OversizedBody(t *testing.T) {
	router, db := newTestRouter(t)
	seedAccount(t, db)
	payload, err := json.Marshal(map[string]string{"email": anaEmail, "password": anaPassword, "padding": oversizedPadding()})
	if err != nil {
		t.Fatalf("no se pudo serializar la solicitud: %v", err)
	}

	rec := postLoginRaw(t, router, string(payload))

	requireBodyTooLarge(t, rec)
	requireNoLoginSideEffects(t, db)
}
