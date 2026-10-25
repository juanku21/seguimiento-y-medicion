package delivery

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/service"
)

// Mensajes de las respuestas de error del registro de esfuerzo, en español y dirigidos a la persona.
// Los de la fecha, del proyecto de solo lectura, del sprint definitivo y del tope diario los fija
// data-model.md sección 7; el del tope lleva las horas disponibles y nunca nombra proyectos. Los del
// cuerpo mal formado, del 400 y del 500 son los mismos que usan la 001 a la 005, y el 500 no expone
// detalles internos.
const (
	msgInvalidFields        = "Hay campos inválidos en la solicitud."
	msgInvalidBody          = "El cuerpo de la solicitud debe ser un JSON válido."
	msgEntryDateInFuture    = "No se registran horas en una fecha futura."
	msgEntryDateBeforeStart = "La fecha no puede ser anterior al inicio del proyecto (AAAA-MM-DD)."
	msgEffortReadOnly       = "El proyecto está finalizado y su esfuerzo es de solo lectura."
	msgSprintFinalized      = "El sprint de la historia ya está cerrado y su esfuerzo es definitivo."
	msgDailyCapExceeded     = "Superarías las 24 horas registradas para ese día. Te quedan %s horas disponibles."
	msgInternalError        = "Ocurrió un error inesperado. Intentá de nuevo más tarde."
)

// Códigos del 409 del registro de esfuerzo (contracts/openapi.yaml, EffortConflictResponse).
const (
	codeReadOnly         = "read_only"
	codeEffortFinalized  = "effort_finalized"
	codeDailyCapExceeded = "daily_cap_exceeded"
)

// storyAccessContextKey es la clave con la que RequireStoryAccess de la 003 deja el StoryAccess en el
// contexto de gin. Replica la de backlog/delivery porque este paquete no lo importa (plan.md,
// Principio III).
const storyAccessContextKey = "backlog.storyAccess"

// Handler reúne los manejadores HTTP del registro de esfuerzo sobre el servicio de esfuerzo.
type Handler struct {
	efforts *service.EffortService
}

// NewHandler compone los manejadores del registro de esfuerzo sobre el servicio recibido.
func NewHandler(efforts *service.EffortService) *Handler {
	return &Handler{efforts: efforts}
}

// createEffortEntry godoc
//
// Los esquemas de error se anotan como delivery.X de auth/delivery, igual que en la 002 a la 005:
// swag los nombra por el nombre del paquete y no por el alias de la importación.
//
//	@Summary		Registra esfuerzo sobre una historia
//	@Description	Registra las horas que el titular de la sesión trabajó sobre la historia en una fecha,
//	@Description	con la actividad realizada (FR-001). El autor es siempre quien inicia la sesión: el
//	@Description	cuerpo no admite indicar a otra persona (FR-002). Las horas son mayores que 0, como
//	@Description	máximo 24 y múltiplos de 0,25 (FR-004); la actividad tiene de 1 a 300 caracteres
//	@Description	(FR-003); la fecha no puede ser futura ni anterior al inicio del proyecto (FR-006). El
//	@Description	registro queda asociado al sprint abierto de la historia o, dentro del plazo de gracia,
//	@Description	al sprint recién cerrado; si no corresponde ninguno, queda sin sprint (FR-012 a
//	@Description	FR-016). La suma de las horas de la persona en la fecha, en todos sus proyectos, no
//	@Description	puede superar 24 (FR-010). Cualquier integrante del proyecto puede registrar esfuerzo.
//	@Tags			Esfuerzo
//	@ID				createEffortEntry
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			storyId	path		string								true	"Identificador de la historia"	format(uuid)
//	@Param			request	body		EffortEntryInput					true	"Datos del esfuerzo a registrar"
//	@Success		201		{object}	EffortEntry							"Registro creado."
//	@Failure		400		{object}	delivery.ValidationErrorResponse	"Entrada inválida, fecha futura o anterior al inicio del proyecto. Indica todos los campos a corregir y por qué, y no crea nada. Si el cuerpo no es un JSON válido o está vacío, la respuesta es ErrorResponse."
//	@Failure		401		{object}	delivery.ErrorResponse				"Se requiere iniciar sesión."
//	@Failure		404		{object}	delivery.ErrorResponse				"La historia no existe o pertenece a un proyecto del que quien pide no es integrante; las dos situaciones devuelven el mismo cuerpo."
//	@Failure		409		{object}	EffortConflictResponse				"Proyecto finished (read_only), sprint de la historia cerrado con el plazo de gracia vencido (effort_finalized) o tope diario de 24 horas superado (daily_cap_exceeded, con las horas disponibles en availableHours)."
//	@Failure		500		{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/stories/{storyId}/effort-entries [post]
func (h *Handler) createEffortEntry(c *gin.Context) {
	// La sesión de la 001 y el RequireStoryAccess de la 003 ya respondieron 401 y 404; si falta el
	// titular o el acceso, la ruta se registró fuera de esos middlewares: es un error de composición
	// y responde 500.
	actorID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		respondInternalError(c, "createEffortEntry sin titular de sesión en el contexto")
		return
	}
	access, ok := storyAccessFromContext(c)
	if !ok {
		respondInternalError(c, "createEffortEntry sin acceso a la historia en el contexto")
		return
	}

	// Se decodifica y valida el cuerpo antes de llegar al servicio: un JSON mal formado responde 400
	// con ErrorResponse y los campos inválidos, 400 con todos a la vez (Principio VII).
	var req EffortEntryInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, authdelivery.ErrorResponse{Error: msgInvalidBody})
		return
	}
	input, fields := ValidateEntryInput(req)
	if len(fields) > 0 {
		respondInvalidFields(c, fields)
		return
	}

	// Se registra el esfuerzo a nombre del titular de la sesión y se traducen los rechazos de dominio
	// a la respuesta del contrato. En el alta el registro nunca es definitivo.
	view, err := h.efforts.Create(c.Request.Context(), access, actorID, input)
	if err != nil {
		respondServiceError(c, err, "no se pudo registrar el esfuerzo")
		return
	}

	c.JSON(http.StatusCreated, newEffortEntry(*view, false))
}

// storyAccessFromContext devuelve el StoryAccess que RequireStoryAccess de la 003 dejó en el contexto
// de gin, y si estaba presente. Si la clave de la 003 cambiara, el alta respondería 500 y las pruebas
// de integración lo detectarían.
func storyAccessFromContext(c *gin.Context) (*backlogdomain.StoryAccess, bool) {
	value, exists := c.Get(storyAccessContextKey)
	if !exists {
		return nil, false
	}
	access, ok := value.(*backlogdomain.StoryAccess)
	return access, ok && access != nil
}

// respondServiceError traduce los errores de dominio del servicio de esfuerzo a la respuesta HTTP del
// contrato: la fecha futura y la anterior al inicio del proyecto son 400 sobre entryDate; el proyecto
// finalizado, el sprint definitivo y el tope diario son 409 con su código, y solo el tope informa las
// horas disponibles. Lo que no es un error de dominio conocido se registra en el log y se responde
// 500 genérico, sin exponer la causa (Principio VII).
func respondServiceError(c *gin.Context, err error, logMessage string) {
	var beforeStart *domain.EntryDateBeforeProjectStartError
	var dailyCap *domain.DailyCapExceededError
	switch {
	case errors.Is(err, domain.ErrEntryDateInFuture):
		respondInvalidFields(c, []authdelivery.FieldError{{Field: "entryDate", Message: msgEntryDateInFuture}})
	case errors.As(err, &beforeStart):
		respondInvalidFields(c, []authdelivery.FieldError{{Field: "entryDate", Message: msgEntryDateBeforeStart}})
	case errors.Is(err, domain.ErrEffortReadOnly):
		c.JSON(http.StatusConflict, EffortConflictResponse{Code: codeReadOnly, Error: msgEffortReadOnly})
	case errors.Is(err, domain.ErrSprintFinalized):
		c.JSON(http.StatusConflict, EffortConflictResponse{Code: codeEffortFinalized, Error: msgSprintFinalized})
	case errors.As(err, &dailyCap):
		available := Hundredths(dailyCap.AvailableHundredths)
		c.JSON(http.StatusConflict, EffortConflictResponse{
			Code:           codeDailyCapExceeded,
			Error:          fmt.Sprintf(msgDailyCapExceeded, formatHours(dailyCap.AvailableHundredths)),
			AvailableHours: &available,
		})
	default:
		respondInternalError(c, logMessage, "error", err)
	}
}

// respondInvalidFields responde el 400 de entrada inválida con los campos recibidos.
func respondInvalidFields(c *gin.Context, fields []authdelivery.FieldError) {
	c.JSON(http.StatusBadRequest, authdelivery.ValidationErrorResponse{Error: msgInvalidFields, Fields: fields})
}

// respondInternalError registra en el log el mensaje y los atributos recibidos y responde el 500
// genérico, sin exponer la causa a quien pide (Principio VII).
func respondInternalError(c *gin.Context, logMessage string, attrs ...any) {
	slog.ErrorContext(c.Request.Context(), logMessage, attrs...)
	c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
}

// formatHours escribe centésimas de hora para el texto dirigido a la persona, con coma decimal y sin
// ceros de más, como el resto de los mensajes de la feature: 400 se escribe 4, 350 se escribe 3,5 y
// 25 se escribe 0,25. El número exacto con 2 decimales viaja aparte, en availableHours.
func formatHours(hundredths int) string {
	whole, fraction := hundredths/100, hundredths%100
	if fraction == 0 {
		return fmt.Sprintf("%d", whole)
	}
	return strings.TrimSuffix(fmt.Sprintf("%d,%02d", whole, fraction), "0")
}
