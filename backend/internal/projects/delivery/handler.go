package delivery

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/service"
)

// Mensajes de las respuestas de error del módulo de proyectos, en español y dirigidos a la persona.
// Los textos del 400, del 401, del 403, del 404, del 409 y del 422 los fija contracts/openapi.yaml; el
// del 500 no expone detalles internos, igual que en la 001.
const (
	msgInvalidFields     = "Hay campos inválidos en la solicitud."
	msgInvalidBody       = "El cuerpo de la solicitud debe ser un JSON válido."
	msgSessionRequired   = "Se requiere iniciar sesión."
	msgProjectNameTaken  = "Ya existe un proyecto propio con ese nombre."
	msgProjectNotFound   = "El proyecto no existe."
	msgNotProjectOwner   = "Solo el propietario del proyecto puede realizar esta acción."
	msgProjectReadOnly   = "El proyecto está finalizado y es de solo lectura."
	msgUserNotRegistered = "No existe un usuario registrado con ese correo electrónico."
	msgAlreadyMember     = "La persona ya forma parte del proyecto."
	msgNotMember         = "La persona no forma parte del proyecto."
	msgOwnerCannotLeave  = "El propietario no puede quitarse del proyecto."
	msgInternalError     = "Ocurrió un error inesperado. Intentá de nuevo más tarde."
)

// Handler agrupa los manejadores HTTP del módulo de proyectos sobre el servicio de proyectos y el
// de integrantes.
type Handler struct {
	projects *service.ProjectService
	members  *service.MemberService
}

// NewHandler crea el conjunto de manejadores con el servicio de proyectos y el de integrantes
// recibidos.
func NewHandler(projects *service.ProjectService, members *service.MemberService) *Handler {
	return &Handler{projects: projects, members: members}
}

// createProject godoc
//
// Los esquemas de error se anotan como delivery.X porque swag los nombra por el nombre del paquete
// de auth/delivery, que es delivery, y no por el alias de la importación.
//
//	@Summary		Crea un proyecto
//	@Description	Crea un proyecto a partir de nombre, descripción opcional, fecha de inicio, fecha de
//	@Description	finalización prevista y factor de horas por Story Point (FR-001). Quien lo crea queda como
//	@Description	propietario y como primer integrante en el mismo acto (FR-007, RN-01). El proyecto nace en
//	@Description	estado planned y sin fecha de finalización real (FR-008). El nombre no se repite entre los
//	@Description	proyectos de un mismo propietario, comparándolo sin distinguir mayúsculas y sin los espacios
//	@Description	extremos (FR-004); el mismo nombre en proyectos de propietarios distintos está permitido. Si
//	@Description	el mismo propietario envía dos solicitudes simultáneas con el mismo nombre, se crea
//	@Description	exactamente un proyecto y la restante recibe 409 (FR-011, SC-007).
//	@Tags			Proyectos
//	@ID				createProject
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		ProjectInput							true	"Datos del proyecto nuevo"
//	@Success		201		{object}	ProjectDetail							"Proyecto creado en estado planned, con quien lo creó como propietario e integrante."
//	@Failure		400		{object}	delivery.ValidationErrorResponse	"Entrada inválida, sin crear ningún proyecto ni membresía. Indica qué campo corregir y por qué (FR-010). Si el cuerpo no es un JSON válido o está vacío, la respuesta es ErrorResponse."
//	@Failure		401		{object}	delivery.ErrorResponse				"Se requiere iniciar sesión (FR-046)."
//	@Failure		409		{object}	delivery.ErrorResponse				"Ya existe un proyecto propio con ese nombre (FR-004, FR-011)."
//	@Failure		500		{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects [post]
func (h *Handler) createProject(c *gin.Context) {
	// La identidad del propietario sale solo de la sesión que verificó el middleware, nunca de la
	// petición. Si no está, la sesión no se verificó y se responde el 401 del contrato.
	ownerID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, authdelivery.ErrorResponse{Error: msgSessionRequired})
		return
	}

	// Se decodifica el cuerpo sin reglas de binding: un cuerpo vacío o mal formado es 400 con
	// ErrorResponse, y las reglas de cada campo las aplica validation.go.
	var req ProjectInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, authdelivery.ErrorResponse{Error: msgInvalidBody})
		return
	}

	// Se validan y normalizan los campos antes de llegar al servicio (Principio VII); ante cualquier
	// campo inválido se responde 400 con su detalle y no se crea nada (FR-010).
	input, fields := buildCreateProjectInput(req)
	if len(fields) > 0 {
		c.JSON(http.StatusBadRequest, authdelivery.ValidationErrorResponse{Error: msgInvalidFields, Fields: fields})
		return
	}

	// Se crea el proyecto y se traducen los errores del servicio: el nombre repetido del mismo
	// propietario es 409 y cualquier otro error es 500 genérico, registrado en el log.
	view, err := h.projects.Create(c.Request.Context(), ownerID, input)
	if errors.Is(err, domain.ErrProjectNameTaken) {
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgProjectNameTaken})
		return
	}
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "no se pudo crear el proyecto", "error", err)
		c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
		return
	}

	c.JSON(http.StatusCreated, newProjectDetail(view, ownerID))
}

// listProjects godoc
//
//	@Summary		Lista los proyectos de los que quien consulta es integrante
//	@Description	Devuelve los proyectos de los que el titular de la sesión es integrante, incluidos los
//	@Description	propios (FR-031). Ningún proyecto ajeno aparece (FR-033). El orden es estable y
//	@Description	predecible: fecha de inicio descendente y, a igual fecha, nombre ascendente (FR-035).
//	@Description	Cuando la persona no participa en ningún proyecto, la respuesta es 200 con una lista
//	@Description	vacía y una indicación clara, no un error (FR-034). No ofrece búsqueda, filtrado ni
//	@Description	paginación.
//	@Tags			Proyectos
//	@ID				listProjects
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	ProjectListResponse		"Listado de proyectos del titular de la sesión."
//	@Failure		401	{object}	delivery.ErrorResponse	"Se requiere iniciar sesión (FR-046)."
//	@Failure		500	{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects [get]
func (h *Handler) listProjects(c *gin.Context) {
	// La identidad sale solo de la sesión verificada por el middleware; sin ella se responde el 401
	// del contrato.
	userID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, authdelivery.ErrorResponse{Error: msgSessionRequired})
		return
	}

	// Se obtiene el listado del servicio; cualquier error es 500 genérico, registrado en el log.
	list, err := h.projects.List(c.Request.Context(), userID)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "no se pudo listar los proyectos", "error", err)
		c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
		return
	}

	c.JSON(http.StatusOK, newProjectListResponse(list))
}

// getProjectDetail godoc
//
//	@Summary		Devuelve el estado completo del proyecto
//	@Description	Devuelve, en una sola respuesta, los datos generales del proyecto, su propietario, el factor
//	@Description	de horas por Story Point, el estado, las fechas, la lista de integrantes, el sprint activo
//	@Description	cuando exista y el resumen del backlog con su equivalente en horas (FR-042). Accesible a
//	@Description	cualquier integrante, incluso si el proyecto está finished (FR-048).
//	@Description	Cuando el proyecto no tiene sprint activo, activeSprint es null y hasActiveSprint es false:
//	@Description	es una situación normal, no un error (FR-043). Sin historias en el backlog, los contadores
//	@Description	por estado, el total de Story Points y las horas estimadas son cero (FR-044).
//	@Description	El correo electrónico de los integrantes se incluye únicamente cuando quien consulta es el
//	@Description	propietario (FR-030). actualEndDate trae la fecha de finalización real (YYYY-MM-DD) cuando
//	@Description	el proyecto está finished y es null en cualquier otro estado (FR-039, FR-040).
//	@Tags			Proyectos
//	@ID				getProjectDetail
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path		string					true	"Identificador del proyecto"	format(uuid)
//	@Success		200			{object}	ProjectDetail			"Estado completo del proyecto."
//	@Failure		401			{object}	delivery.ErrorResponse	"Se requiere iniciar sesión (FR-046)."
//	@Failure		404			{object}	delivery.ErrorResponse	"El proyecto no existe o quien pide no es integrante; las dos situaciones devuelven el mismo cuerpo (FR-049, SC-005)."
//	@Failure		500			{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId} [get]
func (h *Handler) getProjectDetail(c *gin.Context) {
	// La identidad sale solo de la sesión verificada por el middleware; sin ella se responde el 401
	// del contrato. El proyecto sale de la membresía que dejó RequireMembership, que ya descartó al
	// no integrante y al identificador inexistente o mal formado con el 404 (FR-049).
	userID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, authdelivery.ErrorResponse{Error: msgSessionRequired})
		return
	}
	member, ok := membershipFromContext(c)
	if !ok {
		slog.ErrorContext(c.Request.Context(), "getProjectDetail sin membresía en el contexto")
		c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
		return
	}

	// Se arma la vista del proyecto. Si desapareció después de verificar la membresía se responde el
	// mismo 404 indistinguible del middleware; cualquier otro error es 500 genérico, registrado en el
	// log.
	view, err := h.projects.Get(c.Request.Context(), member.ProjectID)
	if errors.Is(err, domain.ErrProjectNotFound) {
		abortProjectNotFound(c)
		return
	}
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "no se pudo obtener el proyecto", "error", err)
		c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
		return
	}

	c.JSON(http.StatusOK, newProjectDetail(view, userID))
}

// addProjectMember godoc
//
//	@Summary		Agrega un integrante al proyecto
//	@Description	Agrega como integrante a un usuario ya registrado, identificándolo por su correo
//	@Description	electrónico (FR-019). No existe forma de sumar a alguien que todavía no tiene cuenta (RC-02).
//	@Description	El correo se normaliza antes de buscar al usuario: se recortan los espacios extremos y se
//	@Description	compara sin distinguir mayúsculas de minúsculas, igual que en la spec 001 (FR-020).
//	@Description	Si el correo no corresponde a ninguna cuenta, la respuesta es 422 y no 404, para que el 404
//	@Description	de este endpoint signifique siempre y solo "el proyecto no existe o no sos integrante"
//	@Description	(FR-021, FR-049, RC-03). Si el usuario ya es integrante, la respuesta es 409 y la membresía
//	@Description	no se duplica (FR-022); dos altas simultáneas del mismo usuario registran una sola
//	@Description	membresía y la restante recibe 409 (FR-023, SC-008). Volver a agregar a alguien que había
//	@Description	sido quitado le devuelve el acceso, con una fecha de incorporación nueva (FR-029).
//	@Description	Solo el propietario, y solo mientras el proyecto no esté finished (FR-018, FR-047).
//	@Tags			Integrantes
//	@ID				addProjectMember
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path		string								true	"Identificador del proyecto"	format(uuid)
//	@Param			request		body		AddMemberRequest					true	"Correo del usuario registrado que se agrega"
//	@Success		201			{object}	ProjectMemberView					"El usuario quedó integrante y desde ese momento puede consultar el proyecto."
//	@Failure		400			{object}	delivery.ValidationErrorResponse	"Entrada inválida: indica qué campo corregir y por qué. Si el cuerpo no es un JSON válido o está vacío, la respuesta es ErrorResponse."
//	@Failure		401			{object}	delivery.ErrorResponse				"Se requiere iniciar sesión (FR-046)."
//	@Failure		403			{object}	delivery.ErrorResponse				"Solo el propietario del proyecto puede realizar esta acción (FR-047, FR-050)."
//	@Failure		404			{object}	delivery.ErrorResponse				"El proyecto no existe o quien pide no es integrante; las dos situaciones devuelven el mismo cuerpo (FR-049, SC-005)."
//	@Failure		409			{object}	delivery.ErrorResponse				"El usuario ya forma parte del proyecto (FR-022, FR-023) o el proyecto está finished (FR-018, RN-11)."
//	@Failure		422			{object}	delivery.ErrorResponse				"El correo no corresponde a ningún usuario registrado (FR-021, RC-03)."
//	@Failure		500			{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId}/members [post]
func (h *Handler) addProjectMember(c *gin.Context) {
	// La identidad sale solo de la sesión y el proyecto de la membresía que dejó RequireMembership;
	// RequireOwnership ya rechazó a quien no es propietario.
	actorID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, authdelivery.ErrorResponse{Error: msgSessionRequired})
		return
	}
	member, ok := membershipFromContext(c)
	if !ok {
		slog.ErrorContext(c.Request.Context(), "addProjectMember sin membresía en el contexto")
		c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
		return
	}

	// Se decodifica el cuerpo y se valida el formato del correo sobre el valor recortado, de modo
	// que los espacios extremos y las mayúsculas no lo invalidan: el servicio lo normaliza (FR-020).
	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, authdelivery.ErrorResponse{Error: msgInvalidBody})
		return
	}
	if err := authdelivery.ValidateEmail(strings.TrimSpace(req.Email)); err != nil {
		c.JSON(http.StatusBadRequest, authdelivery.ValidationErrorResponse{
			Error:  msgInvalidFields,
			Fields: []authdelivery.FieldError{{Field: "email", Message: err.Error()}},
		})
		return
	}

	// Se agrega el integrante y se traducen los errores del servicio a las respuestas del contrato;
	// cualquier otro error es 500 genérico, registrado en el log.
	view, err := h.members.AddMember(c.Request.Context(), member.ProjectID, actorID, req.Email)
	if err != nil {
		respondMemberError(c, err, "no se pudo agregar el integrante")
		return
	}

	// Solo el propietario llega a esta ruta, así que la respuesta incluye el correo (FR-030).
	c.JSON(http.StatusCreated, newProjectMemberView(*view, true))
}

// removeProjectMember godoc
//
//	@Summary		Quita un integrante del proyecto
//	@Description	Quita la membresía de la persona indicada (FR-024). Solo le quita el acceso: el esfuerzo,
//	@Description	los votos de estimación, los defectos y cualquier otro registro que haya generado se
//	@Description	conservan y siguen mostrando su nombre (FR-027, RN-14, SC-011). La persona pierde el acceso
//	@Description	de inmediato y el proyecto pasa a comportarse para ella como inexistente (FR-028).
//	@Description	El propietario no puede quitarse a sí mismo: la respuesta es 409 (FR-025, RN-12).
//	@Description	Si la persona no es integrante del proyecto, o su identificador no es un UUID válido, la
//	@Description	respuesta es 404 (FR-026); el 404 del proyecto se evalúa antes.
//	@Description	Solo el propietario, y solo mientras el proyecto no esté finished (FR-018, FR-047).
//	@Tags			Integrantes
//	@ID				removeProjectMember
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path	string					true	"Identificador del proyecto"									format(uuid)
//	@Param			userId		path	string					true	"Identificador del usuario cuya membresía se quita"	format(uuid)
//	@Success		204			"La persona dejó de ser integrante. Lo que registró en el proyecto se conserva."
//	@Failure		401			{object}	delivery.ErrorResponse	"Se requiere iniciar sesión (FR-046)."
//	@Failure		403			{object}	delivery.ErrorResponse	"Solo el propietario del proyecto puede realizar esta acción (FR-047, FR-050)."
//	@Failure		404			{object}	delivery.ErrorResponse	"El proyecto no existe o quien pide no es integrante (FR-049), o la persona indicada no forma parte del proyecto (FR-026)."
//	@Failure		409			{object}	delivery.ErrorResponse	"El propietario intentó quitarse a sí mismo (FR-025, RN-12) o el proyecto está finished (FR-018, RN-11)."
//	@Failure		500			{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId}/members/{userId} [delete]
func (h *Handler) removeProjectMember(c *gin.Context) {
	// La identidad sale solo de la sesión y el proyecto de la membresía que dejó RequireMembership;
	// RequireOwnership ya rechazó a quien no es propietario.
	actorID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, authdelivery.ErrorResponse{Error: msgSessionRequired})
		return
	}
	member, ok := membershipFromContext(c)
	if !ok {
		slog.ErrorContext(c.Request.Context(), "removeProjectMember sin membresía en el contexto")
		c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
		return
	}

	// Un identificador que no es UUID no puede ser el de un integrante, así que se responde el 404
	// de la persona que no forma parte del proyecto (FR-026).
	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgNotMember})
		return
	}

	// Se quita la membresía y se traducen los errores del servicio a las respuestas del contrato.
	if err := h.members.RemoveMember(c.Request.Context(), member.ProjectID, actorID, userID); err != nil {
		respondMemberError(c, err, "no se pudo quitar el integrante")
		return
	}
	c.Status(http.StatusNoContent)
}

// respondMemberError traduce los errores del servicio de integrantes a la respuesta del contrato. El
// proyecto desaparecido usa la misma salida 404 indistinguible del middleware (FR-049); cualquier
// error no previsto es 500 genérico y se registra en el log con logMessage.
func respondMemberError(c *gin.Context, err error, logMessage string) {
	switch {
	case errors.Is(err, domain.ErrProjectNotFound):
		abortProjectNotFound(c)
	case errors.Is(err, domain.ErrNotProjectOwner):
		c.JSON(http.StatusForbidden, authdelivery.ErrorResponse{Error: msgNotProjectOwner})
	case errors.Is(err, domain.ErrProjectReadOnly):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgProjectReadOnly})
	case errors.Is(err, domain.ErrAlreadyMember):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgAlreadyMember})
	case errors.Is(err, domain.ErrOwnerCannotLeave):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgOwnerCannotLeave})
	case errors.Is(err, domain.ErrUserNotRegistered):
		c.JSON(http.StatusUnprocessableEntity, authdelivery.ErrorResponse{Error: msgUserNotRegistered})
	case errors.Is(err, domain.ErrNotMember):
		c.JSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgNotMember})
	default:
		slog.ErrorContext(c.Request.Context(), logMessage, "error", err)
		c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
	}
}

// newProjectListResponse arma ProjectListResponse a partir del listado del servicio, con las fechas
// como YYYY-MM-DD. La lista se inicializa vacía para que se serialice como [] y nunca como null.
func newProjectListResponse(list *service.ProjectList) ProjectListResponse {
	items := make([]ProjectListItem, 0, len(list.Projects))
	for _, p := range list.Projects {
		items = append(items, ProjectListItem{
			ID:             p.ID.String(),
			Name:           p.Name,
			Status:         p.Status,
			StartDate:      p.StartDate.Format(time.DateOnly),
			PlannedEndDate: p.PlannedEndDate.Format(time.DateOnly),
			MemberCount:    p.MemberCount,
			IsOwner:        p.IsOwner,
		})
	}
	return ProjectListResponse{Projects: items, Message: list.Message}
}

// buildCreateProjectInput aplica las reglas de cada campo de ProjectInput y, si todos son válidos,
// devuelve los datos normalizados para el servicio. Cada campo inválido produce un detalle, en el
// orden name, description, startDate, plannedEndDate, hoursPerStoryPoint. El orden de las fechas
// solo se compara cuando las dos se pudieron leer, y su error se asigna a plannedEndDate (RN-06).
func buildCreateProjectInput(req ProjectInput) (service.CreateProjectInput, []authdelivery.FieldError) {
	fields := []authdelivery.FieldError{}
	addField := func(field string, err error) {
		if err != nil {
			fields = append(fields, authdelivery.FieldError{Field: field, Message: err.Error()})
		}
	}

	addField("name", ValidateProjectName(req.Name))
	addField("description", ValidateDescription(req.Description))
	startDate, startErr := parseCalendarDate(req.StartDate, errInvalidStartDate)
	addField("startDate", startErr)
	plannedEndDate, plannedErr := parseCalendarDate(req.PlannedEndDate, errInvalidPlannedEndDate)
	addField("plannedEndDate", plannedErr)
	if startErr == nil && plannedErr == nil {
		addField("plannedEndDate", ValidateDateOrder(startDate, plannedEndDate))
	}
	hundredths, factorErr := ParseHoursPerStoryPoint(req.HoursPerStoryPoint)
	addField("hoursPerStoryPoint", factorErr)

	return service.CreateProjectInput{
		Name:                         strings.TrimSpace(req.Name),
		NameNormalized:               NormalizeProjectName(req.Name),
		Description:                  NormalizeDescription(req.Description),
		StartDate:                    startDate,
		PlannedEndDate:               plannedEndDate,
		HoursPerStoryPointHundredths: hundredths,
	}, fields
}

// newProjectDetail arma ProjectDetail a partir de la vista del servicio para quien consulta,
// identificado por viewerID. Las fechas se serializan como YYYY-MM-DD y las horas se dividen por 100
// una sola vez (research.md secciones 3 y 4). Cada integrante se arma con newProjectMemberView, que
// incluye el correo solo si quien consulta es el propietario (FR-030); el nombre del propietario se
// toma del integrante marcado como tal. ActualEndDate solo existe cuando el proyecto está finalizado
// y queda nula en cualquier otro estado (FR-008, FR-039).
func newProjectDetail(view *service.ProjectView, viewerID uuid.UUID) ProjectDetail {
	project := view.Project

	viewerIsOwner := viewerID == project.OwnerID
	owner := OwnerRef{ID: project.OwnerID.String()}
	members := make([]ProjectMemberView, 0, len(view.Members))
	for _, m := range view.Members {
		if m.IsOwner {
			owner.FullName = m.FullName
		}
		members = append(members, newProjectMemberView(m, viewerIsOwner))
	}

	// El sprint activo lo informa el puerto de sprints; su ausencia se indica también con
	// HasActiveSprint en falso (FR-043).
	var activeSprint *ActiveSprintRef
	if view.ActiveSprint != nil {
		activeSprint = &ActiveSprintRef{ID: view.ActiveSprint.ID.String(), Name: view.ActiveSprint.Name}
	}

	// La fecha de finalización real se copia con el mismo formato que las demás fechas y se omite
	// (null) cuando el proyecto todavía no terminó (FR-039, FR-040).
	var actualEndDate *string
	if project.ActualEndDate != nil {
		formatted := project.ActualEndDate.Format(time.DateOnly)
		actualEndDate = &formatted
	}

	return ProjectDetail{
		ID:                 project.ID.String(),
		Name:               project.Name,
		Description:        project.Description,
		Status:             project.Status,
		StartDate:          project.StartDate.Format(time.DateOnly),
		PlannedEndDate:     project.PlannedEndDate.Format(time.DateOnly),
		ActualEndDate:      actualEndDate,
		HoursPerStoryPoint: hoursFromHundredths(project.HoursPerStoryPointHundredths),
		ReadOnly:           view.ReadOnly,
		Owner:              owner,
		Members:            members,
		HasActiveSprint:    activeSprint != nil,
		ActiveSprint:       activeSprint,
		Backlog: BacklogSummary{
			CountsByState:        view.Backlog.CountsByState,
			EstimatedStoryPoints: view.Backlog.EstimatedStoryPoints,
			EstimatedHours:       hoursFromHundredths(view.EstimatedHoursHundredths),
		},
	}
}
