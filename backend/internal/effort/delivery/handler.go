package delivery

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

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
	msgNotEntryAuthor       = "Solo quien registró este esfuerzo puede modificarlo o eliminarlo."
	msgEffortFinalized      = "Este esfuerzo pertenece a un sprint cerrado y ya es definitivo."
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

// getStoryEffort godoc
//
//	@Summary		Consulta el esfuerzo de una historia
//	@Description	Devuelve el total de horas reales de la historia, el desglose por integrante y la
//	@Description	lista de registros (FR-030 a FR-033). El total incluye todos los registros, de
//	@Description	cualquier autor, fecha y sprint, incluidos los sin sprint (FR-031). Las personas que
//	@Description	ya no son integrantes aparecen con su nombre (FR-032). La lista se ordena por fecha
//	@Description	descendente y, a igual fecha, por momento de creación (FR-035); el desglose, por
//	@Description	nombre. Una historia sin registros devuelve total 0.00 y listas vacías, no un error
//	@Description	(FR-034). Cualquier integrante del proyecto puede consultarlo.
//	@Tags			Esfuerzo
//	@ID				getStoryEffort
//	@Produce		json
//	@Security		BearerAuth
//	@Param			storyId	path		string					true	"Identificador de la historia"	format(uuid)
//	@Success		200		{object}	StoryEffort				"Esfuerzo de la historia, posiblemente vacío."
//	@Failure		401		{object}	delivery.ErrorResponse	"Se requiere iniciar sesión."
//	@Failure		404		{object}	delivery.ErrorResponse	"La historia no existe o pertenece a un proyecto del que quien pide no es integrante; las dos situaciones devuelven el mismo cuerpo."
//	@Failure		500		{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/stories/{storyId}/effort [get]
func (h *Handler) getStoryEffort(c *gin.Context) {
	// RequireStoryAccess de la 003 ya respondió 404 a la historia inexistente o ajena; si falta el
	// acceso, la ruta se registró fuera de ese middleware: es un error de composición y responde 500.
	access, ok := storyAccessFromContext(c)
	if !ok {
		respondInternalError(c, "getStoryEffort sin acceso a la historia en el contexto")
		return
	}

	// Se consulta el esfuerzo de la historia y cualquier falla responde 500 sin exponer la causa.
	effort, err := h.efforts.StoryEffort(c.Request.Context(), access.StoryID)
	if err != nil {
		respondInternalError(c, "no se pudo consultar el esfuerzo de la historia", "error", err)
		return
	}

	c.JSON(http.StatusOK, newStoryEffort(access.StoryID, *effort))
}

// updateEffortEntry godoc
//
//	@Summary		Corrige un registro propio
//	@Description	Modifica la fecha, la actividad o las horas de un registro propio (FR-018). Todos los
//	@Description	campos son opcionales, pero debe venir al menos uno. Se aplican las mismas validaciones
//	@Description	que en el alta (FR-023); el tope diario se evalúa sobre la fecha resultante excluyendo
//	@Description	las horas vigentes del propio registro (FR-024). La modificación es atómica (FR-025).
//	@Description	El autor, la historia y el sprint asociado no cambian: enviar storyId, userId o
//	@Description	sprintId se rechaza con 400 (FR-019). Se rechaza con 403 si el registro es de otra
//	@Description	persona, aunque quien lo intente sea el propietario del proyecto (FR-022), y con 409 si
//	@Description	el proyecto está finished o si el sprint asociado está closed con el plazo de gracia
//	@Description	vencido (FR-020, FR-027). Una persona quitada del proyecto recibe 404 sobre sus
//	@Description	registros anteriores (FR-028).
//	@Tags			Esfuerzo
//	@ID				updateEffortEntry
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			entryId	path		string								true	"Identificador del registro de esfuerzo"	format(uuid)
//	@Param			request	body		EffortEntryPatch					true	"Campos a corregir"
//	@Success		200		{object}	EffortEntry							"Registro actualizado."
//	@Failure		400		{object}	delivery.ValidationErrorResponse	"Entrada inválida, campo inmutable, cuerpo sin campos editables, fecha futura o anterior al inicio del proyecto. Indica todos los campos a corregir y por qué, y no modifica nada. Si el cuerpo no es un JSON válido o está vacío, la respuesta es ErrorResponse."
//	@Failure		401		{object}	delivery.ErrorResponse				"Se requiere iniciar sesión."
//	@Failure		403		{object}	delivery.ErrorResponse				"El registro es de otra persona («Solo quien registró este esfuerzo puede modificarlo o eliminarlo.», FR-022)."
//	@Failure		404		{object}	delivery.ErrorResponse				"El registro no existe, el identificador está mal formado o pertenece a un proyecto del que quien pide no es integrante; las situaciones devuelven el mismo cuerpo."
//	@Failure		409		{object}	EffortConflictResponse				"Proyecto finished (read_only), registro de un sprint cerrado con el plazo de gracia vencido (effort_finalized) o tope diario de 24 horas superado (daily_cap_exceeded, con las horas disponibles en availableHours)."
//	@Failure		500		{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/effort-entries/{entryId} [patch]
func (h *Handler) updateEffortEntry(c *gin.Context) {
	// La sesión de la 001 y RequireEffortEntryAccess ya respondieron 401 y 404; si falta el titular o
	// el acceso, la ruta se registró fuera de esos middlewares: es un error de composición y responde
	// 500.
	actorID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		respondInternalError(c, "updateEffortEntry sin titular de sesión en el contexto")
		return
	}
	access, ok := entryAccessFromContext(c)
	if !ok || access == nil {
		respondInternalError(c, "updateEffortEntry sin acceso al registro en el contexto")
		return
	}

	// Se decodifica y valida el parche antes de llegar al servicio: un JSON mal formado responde 400
	// con ErrorResponse y los campos inválidos, 400 con todos a la vez (Principio VII).
	var req EffortEntryPatch
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, authdelivery.ErrorResponse{Error: msgInvalidBody})
		return
	}
	patch, fields := ValidateEntryPatch(req)
	if len(fields) > 0 {
		respondInvalidFields(c, fields)
		return
	}

	// Se corrige el registro a nombre del titular de la sesión y se traducen los rechazos de dominio
	// a la respuesta del contrato. Un registro que admitió la corrección no es definitivo.
	view, err := h.efforts.Update(c.Request.Context(), access.EntryID, actorID, patch)
	if err != nil {
		respondServiceError(c, err, "no se pudo corregir el esfuerzo")
		return
	}

	c.JSON(http.StatusOK, newEffortEntry(*view, false))
}

// deleteEffortEntry godoc
//
//	@Summary		Elimina un registro propio
//	@Description	Elimina definitivamente un registro propio; deja de contar de inmediato en los totales
//	@Description	de la historia, del sprint, del proyecto y del tope diario de esa fecha (FR-026). La
//	@Description	historia sigue sin poder eliminarse aunque se borren todos sus registros (RN-22).
//	@Description	Mismos rechazos que la modificación: 403 por registro ajeno; 409 por proyecto finished
//	@Description	o sprint closed con el plazo vencido; 404 para quien ya no es integrante.
//	@Tags			Esfuerzo
//	@ID				deleteEffortEntry
//	@Produce		json
//	@Security		BearerAuth
//	@Param			entryId	path	string	true	"Identificador del registro de esfuerzo"	format(uuid)
//	@Success		204		"Registro eliminado."
//	@Failure		401		{object}	delivery.ErrorResponse	"Se requiere iniciar sesión."
//	@Failure		403		{object}	delivery.ErrorResponse	"El registro es de otra persona («Solo quien registró este esfuerzo puede modificarlo o eliminarlo.», FR-022)."
//	@Failure		404		{object}	delivery.ErrorResponse	"El registro no existe, el identificador está mal formado o pertenece a un proyecto del que quien pide no es integrante; las situaciones devuelven el mismo cuerpo."
//	@Failure		409		{object}	EffortConflictResponse	"Proyecto finished (read_only) o registro de un sprint cerrado con el plazo de gracia vencido (effort_finalized)."
//	@Failure		500		{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/effort-entries/{entryId} [delete]
func (h *Handler) deleteEffortEntry(c *gin.Context) {
	// La sesión de la 001 y RequireEffortEntryAccess ya respondieron 401 y 404; si falta el titular o
	// el acceso, es un error de composición y responde 500.
	actorID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		respondInternalError(c, "deleteEffortEntry sin titular de sesión en el contexto")
		return
	}
	access, ok := entryAccessFromContext(c)
	if !ok || access == nil {
		respondInternalError(c, "deleteEffortEntry sin acceso al registro en el contexto")
		return
	}

	// Se elimina el registro a nombre del titular de la sesión y se traducen los rechazos de dominio
	// a la respuesta del contrato; el 204 no lleva cuerpo.
	if err := h.efforts.Delete(c.Request.Context(), access.EntryID, actorID); err != nil {
		respondServiceError(c, err, "no se pudo eliminar el esfuerzo")
		return
	}

	c.Status(http.StatusNoContent)
}

// listProjectEffort godoc
//
//	@Summary		Lista el esfuerzo del proyecto con filtros
//	@Description	Devuelve los registros del proyecto que cumplen todos los filtros indicados y el total
//	@Description	de horas del conjunto (FR-036, FR-037, FR-041). Orden: fecha descendente y, a igual
//	@Description	fecha, por momento de creación (FR-045). Sin paginación. El filtro por sprint opera
//	@Description	sobre el sprint asociado al registro, no sobre el sprint en el que esté hoy la
//	@Description	historia, y admite none para los registros sin sprint (FR-039). El filtro por
//	@Description	integrante admite a personas que ya no son integrantes, si dejaron registros en el
//	@Description	proyecto (FR-040). El rango de fechas es inclusivo en ambos extremos (FR-038). Un
//	@Description	filtro mal formado, inexistente o de otro proyecto se rechaza con 400 y el mismo
//	@Description	mensaje, sin revelar datos del otro proyecto (FR-044). Sin coincidencias, la respuesta
//	@Description	es una lista vacía con total 0.00 y el motivo en emptyReason, no un error (FR-042).
//	@Tags			Esfuerzo
//	@ID				listProjectEffort
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path		string								true	"Identificador del proyecto"	format(uuid)
//	@Param			userId		query		string								false	"Autor de los registros: integrante vigente o persona con registros en el proyecto"	format(uuid)
//	@Param			storyId		query		string								false	"Historia del proyecto"	format(uuid)
//	@Param			sprintId	query		string								false	"Sprint del proyecto asociado a los registros, o none para los registros sin sprint"
//	@Param			from		query		string								false	"Fecha mínima, inclusiva (AAAA-MM-DD)"	format(date)
//	@Param			to			query		string								false	"Fecha máxima, inclusiva (AAAA-MM-DD); no puede ser anterior a from (FR-043)"	format(date)
//	@Success		200			{object}	ProjectEffort						"Registros filtrados con su total, posiblemente vacíos."
//	@Failure		400			{object}	delivery.ValidationErrorResponse	"Algún filtro es inválido: fecha mal formada, rango invertido o filtro mal formado, inexistente o de otro proyecto («El filtro no corresponde al proyecto.»)."
//	@Failure		401			{object}	delivery.ErrorResponse				"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse				"El proyecto no existe o quien pide no es integrante; las dos situaciones devuelven el mismo cuerpo."
//	@Failure		500			{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId}/effort-entries [get]
func (h *Handler) listProjectEffort(c *gin.Context) {
	// El subgrupo de la 002 ya respondió 401 y 404; un projectId que no sea UUID solo es posible si
	// la ruta se registró fuera de ese subgrupo: es un error de composición y responde 500.
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respondInternalError(c, "listProjectEffort sin un projectId válido en la ruta", "error", err)
		return
	}

	// Se validan los filtros antes de llegar al servicio: los mal formados responden 400 con todos los
	// campos a la vez (Principio VII).
	filter, fields := ParseEntryFilter(c.Request.URL.Query())
	if len(fields) > 0 {
		respondInvalidFields(c, fields)
		return
	}

	// Se consulta el esfuerzo filtrado. Un filtro inexistente o de otro proyecto responde el mismo 400
	// que uno mal formado, con solo el campo rechazado y sin datos del otro proyecto (FR-044).
	view, err := h.efforts.ProjectEffort(c.Request.Context(), projectID, filter)
	if err != nil {
		var invalidFilter *domain.InvalidFilterError
		if errors.As(err, &invalidFilter) {
			respondInvalidFields(c, []authdelivery.FieldError{{Field: invalidFilter.Field, Message: msgFilterNotInProject}})
			return
		}
		respondInternalError(c, "no se pudo consultar el esfuerzo del proyecto", "error", err)
		return
	}

	c.JSON(http.StatusOK, newProjectEffort(*view))
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
// contrato: la fecha futura y la anterior al inicio del proyecto son 400 sobre entryDate; el registro
// ajeno es 403; el registro que desapareció después del middleware es el mismo 404 de este; el
// proyecto finalizado, el sprint definitivo, el registro definitivo y el tope diario son 409 con su
// código, y solo el tope informa las horas disponibles. Lo que no es un error de dominio conocido se
// registra en el log y se responde 500 genérico, sin exponer la causa (Principio VII).
func respondServiceError(c *gin.Context, err error, logMessage string) {
	var beforeStart *domain.EntryDateBeforeProjectStartError
	var dailyCap *domain.DailyCapExceededError
	switch {
	case errors.Is(err, domain.ErrNotEntryAuthor):
		c.JSON(http.StatusForbidden, authdelivery.ErrorResponse{Error: msgNotEntryAuthor})
	case errors.Is(err, domain.ErrEntryNotFound):
		c.JSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgEntryNotFound})
	case errors.Is(err, domain.ErrEffortFinalized):
		c.JSON(http.StatusConflict, EffortConflictResponse{Code: codeEffortFinalized, Error: msgEffortFinalized})
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
