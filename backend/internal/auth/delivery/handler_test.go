package delivery

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/service"
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

// newTestRouter abre la base de test vacía y compone la cadena real repositorio, servicio,
// manejadores y rutas, igual que cmd/app, sin CORS ni Swagger.
func newTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, _ := testsupport.OpenDatabase(t)
	handler := NewHandler(service.NewAuthService(repository.NewUserRepository(db)))
	router := gin.New()
	RegisterRoutes(router, handler)
	return router, db
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
// login_succeeded (FR-009). El acceso rechazado a una función protegida se verifica en US3/US4,
// cuando esas funciones existen.
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
