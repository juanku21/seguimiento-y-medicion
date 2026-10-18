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
// Los textos del 400, del 401, del 403, del 404 y del 409 los fija contracts/openapi.yaml; el del 500 no expone
// detalles internos, igual que en la 001.
const (
	msgInvalidFields    = "Hay campos inválidos en la solicitud."
	msgInvalidBody      = "El cuerpo de la solicitud debe ser un JSON válido."
	msgSessionRequired  = "Se requiere iniciar sesión."
	msgProjectNameTaken = "Ya existe un proyecto propio con ese nombre."
	msgProjectNotFound  = "El proyecto no existe."
	msgNotProjectOwner  = "Solo el propietario del proyecto puede realizar esta acción."
	msgInternalError    = "Ocurrió un error inesperado. Intentá de nuevo más tarde."
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
//	@Description	propietario (FR-030). Por ahora actualEndDate se informa siempre como null.
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
// toma del integrante marcado como tal. Un proyecto recién creado no tiene fecha de finalización
// real, por eso ActualEndDate queda nula (FR-008).
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

	return ProjectDetail{
		ID:                 project.ID.String(),
		Name:               project.Name,
		Description:        project.Description,
		Status:             project.Status,
		StartDate:          project.StartDate.Format(time.DateOnly),
		PlannedEndDate:     project.PlannedEndDate.Format(time.DateOnly),
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
