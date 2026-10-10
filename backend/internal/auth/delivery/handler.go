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

// Mensajes generales de las respuestas de error del registro, en español y dirigidos a la persona.
// Los textos del 400 y del 409 los fija contracts/openapi.yaml; el del 500 no expone detalles
// internos (FR-011).
const (
	msgInvalidFields    = "Hay campos inválidos en la solicitud."
	msgInvalidBody      = "El cuerpo de la solicitud debe ser un JSON válido."
	msgEmailUnavailable = "El correo electrónico no está disponible."
	msgInternalError    = "Ocurrió un error inesperado. Intentá de nuevo más tarde."
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
//	@Failure		400		{object}	ValidationErrorResponse	"Entrada inválida: indica qué campo corregir y por qué, sin crear ninguna cuenta (FR-008)."
//	@Failure		409		{object}	ErrorResponse			"El correo normalizado ya está registrado (FR-005, FR-007)."
//	@Failure		500		{object}	ErrorResponse			"Error interno sin detalles expuestos."
//	@Router			/api/v1/auth/register [post]
func (h *Handler) registerUser(c *gin.Context) {
	// Se decodifica el cuerpo sin reglas de binding: un JSON mal formado o con tipos incorrectos se
	// rechaza con 400 descriptivo, y las reglas de cada campo las aplica validation.go.
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: msgInvalidBody})
		return
	}

	// Se normaliza el correo y se valida cada campo, acumulando un detalle por campo inválido
	// (FR-008). El nombre se guarda recortado, igual que se midió al validarlo (RN-03).
	fullName := strings.TrimSpace(req.FullName)
	email := NormalizeEmail(req.Email)
	fields := []FieldError{}
	if err := ValidateFullName(fullName); err != nil {
		fields = append(fields, FieldError{Field: "fullName", Message: err.Error()})
	}
	if err := ValidateEmail(email); err != nil {
		fields = append(fields, FieldError{Field: "email", Message: err.Error()})
	}
	if err := ValidatePassword(req.Password); err != nil {
		fields = append(fields, FieldError{Field: "password", Message: err.Error()})
	}
	if len(fields) > 0 {
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

// newUserProfile arma los datos visibles de la cuenta, sin la contraseña ni su hash (FR-011).
func newUserProfile(user *domain.User) UserProfile {
	return UserProfile{
		ID:       user.ID.String(),
		FullName: user.FullName,
		Email:    user.Email,
	}
}
