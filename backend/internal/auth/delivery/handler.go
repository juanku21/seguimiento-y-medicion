package delivery

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/service"
)

// Mensajes de las respuestas de error del registro y del inicio de sesión, en español y dirigidos
// a la persona. Los textos del 400, del 401 y del 409 los fija contracts/openapi.yaml; el del 500
// no expone detalles internos (FR-011). El 401 es único para correo inexistente y contraseña
// incorrecta (FR-014, RN-06).
const (
	msgInvalidFields         = "Hay campos inválidos en la solicitud."
	msgInvalidBody           = "El cuerpo de la solicitud debe ser un JSON válido."
	msgEmailUnavailable      = "El correo electrónico no está disponible."
	msgInvalidCredentials    = "Correo electrónico o contraseña incorrectos."
	msgInternalError         = "Ocurrió un error inesperado. Intentá de nuevo más tarde."
	msgLoginEmailRequired    = "El correo electrónico es obligatorio."
	msgLoginPasswordRequired = "La contraseña es obligatoria."
)

// Handler agrupa los manejadores HTTP de la feature de autenticación sobre el servicio.
type Handler struct {
	auth *service.AuthService
}

// NewHandler crea el conjunto de manejadores con el servicio de autenticación recibido.
func NewHandler(auth *service.AuthService) *Handler {
	return &Handler{auth: auth}
}

// registerUser godoc
//
//	@Summary		Registra una cuenta nueva
//	@Description	Crea una cuenta a partir de nombre completo, correo electrónico y contraseña (FR-001).
//	@Description	El correo se normaliza recortando los espacios extremos y pasándolo a minúsculas (FR-004)
//	@Description	y debe ser único en todo el sistema (FR-005, FR-007). El registro no emite sesión (FR-009):
//	@Description	al completarlo hay que iniciar sesión con las credenciales recién creadas.
//	@Tags			Autenticación
//	@ID				registerUser
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RegisterRequest			true	"Datos de la cuenta nueva"
//	@Success		201		{object}	UserProfile				"Cuenta creada. La persona ya puede iniciar sesión con ese correo."
//	@Failure		400		{object}	ValidationErrorResponse	"Entrada inválida, sin crear ninguna cuenta. Tiene dos formas: ValidationErrorResponse cuando uno o más campos no cumplen las reglas, con qué campo corregir y por qué (FR-008); ErrorResponse cuando el cuerpo no es un JSON válido o está vacío."
//	@Failure		409		{object}	ErrorResponse			"El correo normalizado ya está registrado (FR-005, FR-007)."
//	@Failure		500		{object}	ErrorResponse			"Error interno sin detalles expuestos."
//	@Router			/api/v1/auth/register [post]
func (h *Handler) registerUser(c *gin.Context) {
	// Se decodifica el cuerpo sin reglas de binding: un cuerpo vacío, mal formado o con tipos incorrectos se
	// rechaza con 400 y ErrorResponse, y las reglas de cada campo las aplica validation.go.
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msgInvalidBody})
		return
	}

	// Se normaliza el correo y se validan los campos; ante cualquier campo inválido se responde 400
	// con su detalle (FR-008). El nombre se guarda recortado, igual que se midió al validarlo (RN-03).
	fullName := strings.TrimSpace(req.FullName)
	email := NormalizeEmail(req.Email)
	if fields := validateRegistration(fullName, email, req.Password); len(fields) > 0 {
		c.JSON(http.StatusBadRequest, ValidationErrorResponse{Error: msgInvalidFields, Fields: fields})
		return
	}

	// Se crea la cuenta y se traducen los errores del servicio: el correo duplicado es 409 y
	// cualquier otro error es 500 genérico, registrado en el log sin la contraseña (FR-011).
	user, err := h.auth.Register(c.Request.Context(), fullName, email, req.Password)
	if errors.Is(err, domain.ErrEmailUnavailable) {
		c.JSON(http.StatusConflict, ErrorResponse{Error: msgEmailUnavailable})
		return
	}
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "no se pudo registrar la cuenta", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: msgInternalError})
		return
	}

	c.JSON(http.StatusCreated, newUserProfile(user))
}

// loginUser godoc
//
//	@Summary		Inicia sesión y emite una sesión vigente
//	@Description	Valida las credenciales y devuelve una sesión con momento de vencimiento (FR-012, FR-013).
//	@Description	Al correo se le aplica la misma normalización del registro (FR-004).
//	@Description	La sesión vence 8 horas después de su emisión y el plazo es fijo: no se renueva ni se
//	@Description	extiende por actividad (FR-018). Una misma cuenta puede tener varias sesiones vigentes en
//	@Description	distintos dispositivos sin que unas invaliden a otras (FR-017).
//	@Tags			Autenticación
//	@ID				loginUser
//	@Accept			json
//	@Produce		json
//	@Param			request	body		LoginRequest			true	"Credenciales de la cuenta"
//	@Success		200		{object}	LoginResponse			"Sesión emitida."
//	@Failure		400		{object}	ValidationErrorResponse	"Falta el correo o la contraseña en la solicitud. Si el cuerpo no es un JSON válido o está vacío, la respuesta es ErrorResponse."
//	@Failure		401		{object}	ErrorResponse			"Credenciales inválidas. El mensaje es idéntico tanto si el correo no existe como si la contraseña es incorrecta (FR-014, RN-06, SC-004)."
//	@Failure		500		{object}	ErrorResponse			"Error interno sin detalles expuestos."
//	@Router			/api/v1/auth/login [post]
func (h *Handler) loginUser(c *gin.Context) {
	// Se decodifica el cuerpo igual que en el registro: un cuerpo vacío o mal formado es 400 con
	// ErrorResponse.
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msgInvalidBody})
		return
	}

	// Se normaliza el correo antes de llegar al servicio (research.md sección 3) y se exige que
	// estén presentes el correo y la contraseña, única regla de entrada que fija el contrato.
	email := NormalizeEmail(req.Email)
	if fields := validateLogin(email, req.Password); len(fields) > 0 {
		c.JSON(http.StatusBadRequest, ValidationErrorResponse{Error: msgInvalidFields, Fields: fields})
		return
	}

	// Se inicia la sesión y se traducen los errores del servicio: las credenciales inválidas son
	// 401 con el mensaje genérico y cualquier otro error es 500, registrado sin la contraseña.
	result, err := h.auth.Login(c.Request.Context(), email, req.Password)
	if errors.Is(err, domain.ErrInvalidCredentials) {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: msgInvalidCredentials})
		return
	}
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "no se pudo iniciar la sesión", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: msgInternalError})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{Token: result.Token, ExpiresAt: result.ExpiresAt})
}

// getOwnProfile godoc
//
//	@Summary		Devuelve el perfil del titular de la sesión
//	@Description	Devuelve el nombre completo y el correo electrónico del titular de la sesión con la que se
//	@Description	hizo la petición (FR-022). No existe forma de consultar el perfil de otra cuenta: no se
//	@Description	acepta ningún identificador como parámetro (FR-023). El correo se devuelve tal como quedó
//	@Description	normalizado, y la respuesta nunca incluye la contraseña ni ninguna representación derivada
//	@Description	de ella (FR-011).
//	@Tags			Usuarios
//	@ID				getOwnProfile
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	UserProfile		"Perfil del titular de la sesión."
//	@Failure		401	{object}	ErrorResponse	"Se requiere iniciar sesión. Misma respuesta para sesión ausente, vencida, cerrada por su titular o adulterada (FR-015, FR-016, RN-07)."
//	@Failure		500	{object}	ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/users/me [get]
func (h *Handler) getOwnProfile(c *gin.Context) {
	// La identidad sale solo del sub que dejó el middleware de sesión, nunca de la petición
	// (FR-023). Si no está, la sesión no se verificó y se responde el 401 del contrato.
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: msgSessionRequired})
		return
	}

	// Se obtiene la cuenta y se traducen los errores del servicio: una cuenta inexistente es sesión
	// inválida (401) y cualquier otro error es 500 genérico, registrado en el log.
	user, err := h.auth.GetOwnProfile(c.Request.Context(), userID)
	if errors.Is(err, domain.ErrInvalidSession) {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: msgSessionRequired})
		return
	}
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "no se pudo obtener el perfil", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: msgInternalError})
		return
	}

	c.JSON(http.StatusOK, newUserProfile(user))
}

// validateLogin exige la presencia del correo ya normalizado y de la contraseña, y devuelve un
// detalle por cada campo faltante, en el orden email, password. No aplica las reglas de formato
// del registro: una credencial que no las cumpla simplemente no coincide con ninguna cuenta.
func validateLogin(email, password string) []FieldError {
	fields := []FieldError{}
	if email == "" {
		fields = append(fields, FieldError{Field: "email", Message: msgLoginEmailRequired})
	}
	if password == "" {
		fields = append(fields, FieldError{Field: "password", Message: msgLoginPasswordRequired})
	}
	return fields
}

// validateRegistration aplica las reglas de cada campo del registro y devuelve un detalle por cada
// campo inválido, en el orden fullName, email, password. Si todos son válidos, la lista queda vacía.
func validateRegistration(fullName, email, password string) []FieldError {
	checks := []struct {
		field string
		err   error
	}{
		{field: "fullName", err: ValidateFullName(fullName)},
		{field: "email", err: ValidateEmail(email)},
		{field: "password", err: ValidatePassword(password)},
	}
	fields := []FieldError{}
	for _, check := range checks {
		if check.err != nil {
			fields = append(fields, FieldError{Field: check.field, Message: check.err.Error()})
		}
	}
	return fields
}

// newUserProfile arma los datos visibles de la cuenta, sin la contraseña ni su hash (FR-011).
func newUserProfile(user *domain.User) UserProfile {
	return UserProfile{
		ID:       user.ID.String(),
		FullName: user.FullName,
		Email:    user.Email,
	}
}
