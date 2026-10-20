package delivery

import (
	"encoding/json"
	"time"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// Los esquemas de error ErrorResponse, FieldError y ValidationErrorResponse no se redeclaran acá:
// se reutilizan tal cual desde auth/delivery de la feature 001 (research.md sección 12).

// ProjectInput es el cuerpo de la creación de un proyecto (contracts/openapi.yaml). Las etiquetas
// validate y de longitud solo documentan el contrato en Swagger: la validación real la hacen las
// funciones de validation.go, que devuelven los mensajes por campo (FR-010). Las fechas llegan como
// texto YYYY-MM-DD y el factor como json.Number, para contar sus decimales sobre el texto literal
// sin redondear (research.md sección 3). Un campo ausente queda en su valor cero y lo rechaza la
// validación, de modo que se puede indicar qué campo falta.
type ProjectInput struct {
	// Nombre del proyecto, entre 1 y 100 caracteres tras recortar los espacios extremos (FR-002).
	Name string `json:"name" validate:"required" minLength:"1" maxLength:"100" example:"Portal de Clientes"`
	// Descripción opcional de hasta 1000 caracteres; vacía o solo de espacios se guarda como ausente (FR-003).
	Description *string `json:"description" maxLength:"1000" extensions:"x-nullable" example:"Rediseño del portal de autogestión."`
	// Fecha de inicio como fecha de calendario sin hora (YYYY-MM-DD).
	StartDate string `json:"startDate" validate:"required" format:"date" example:"2026-10-01"`
	// Fecha de finalización prevista, nunca anterior a la de inicio; iguales es válido (FR-005, RN-06).
	PlannedEndDate string `json:"plannedEndDate" validate:"required" format:"date" example:"2026-12-20"`
	// Factor de horas por Story Point: mayor que 0, menor o igual a 40 y con hasta 2 decimales (FR-006, RN-07).
	HoursPerStoryPoint json.Number `json:"hoursPerStoryPoint" validate:"required" swaggertype:"number" maximum:"40" multipleOf:"0.01" example:"6.5"`
}

// OwnerRef identifica al propietario del proyecto, único y no transferible (RC-04).
type OwnerRef struct {
	// Identificador de la cuenta del propietario.
	ID string `json:"id" validate:"required" format:"uuid" example:"3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04"`
	// Nombre completo del propietario.
	FullName string `json:"fullName" validate:"required" example:"Ana Pérez"`
}

// AddMemberRequest es el cuerpo del alta de un integrante (contracts/openapi.yaml). Las etiquetas
// validate y de longitud solo documentan el contrato en Swagger. El correo llega tal como lo
// escribió la persona: el servicio lo normaliza con TrimSpace y ToLower antes de buscar la cuenta
// (FR-020, RN-05), por eso acá no se recorta ni se rechaza por mayúsculas o espacios extremos.
type AddMemberRequest struct {
	// Correo electrónico de un usuario ya registrado (FR-019, RC-02).
	Email string `json:"email" validate:"required" format:"email" maxLength:"254" example:"bruno@example.com"`
}

// StatusChangeRequest es el cuerpo del cambio de estado del proyecto (contracts/openapi.yaml). La
// etiqueta enums solo documenta el contrato en Swagger: el manejador rechaza con 400 cualquier valor
// fuera de los tres estados antes de llegar al servicio.
type StatusChangeRequest struct {
	// Estado destino del proyecto (FR-036, FR-037).
	Status domain.ProjectStatus `json:"status" validate:"required" enums:"planned,in_progress,finished" example:"in_progress"`
}

// ProjectMemberView es un integrante del proyecto (FR-030). El correo se omite del JSON cuando
// quien consulta no es el propietario: newProjectMemberView lo deja vacío en ese caso, y una cuenta
// siempre tiene correo, así que el valor vacío no se confunde con un dato real.
type ProjectMemberView struct {
	// Identificador de la cuenta del integrante.
	UserID string `json:"userId" validate:"required" format:"uuid" example:"7c2d4e81-0a3b-4f6c-8d9e-1b2a3c4d5e6f"`
	// Nombre completo del integrante.
	FullName string `json:"fullName" validate:"required" example:"Bruno Gómez"`
	// Correo normalizado; presente solo si quien consulta es el propietario (FR-030).
	Email string `json:"email,omitempty" format:"email" example:"bruno@example.com"`
	// Marca de propietario, derivada de comparar al integrante con el propietario del proyecto.
	IsOwner bool `json:"isOwner" validate:"required" example:"false"`
	// Momento en que la persona se incorporó al proyecto.
	JoinedAt string `json:"joinedAt" validate:"required" format:"date-time" example:"2026-10-03T09:15:00Z"`
}

// ActiveSprintRef es el sprint activo del proyecto, que produce la spec 004 y esta feature solo
// informa (RC-06).
type ActiveSprintRef struct {
	// Identificador del sprint.
	ID string `json:"id" validate:"required" format:"uuid" example:"9e8d7c6b-5a49-4382-9170-6f5e4d3c2b1a"`
	// Nombre del sprint.
	Name string `json:"name" validate:"required" example:"Sprint 4"`
}

// BacklogSummary es el resumen del backlog que produce la spec 003, con las horas calculadas con
// el factor del proyecto (FR-042 a FR-045). CountsByState se serializa como objeto y nunca como
// null: sin historias va vacío (FR-044).
type BacklogSummary struct {
	// Cantidad de historias por cada estado que defina la feature de backlog.
	CountsByState map[string]int `json:"countsByState" validate:"required"`
	// Total de Story Points de las historias estimadas; las sin estimar no suman (FR-045).
	EstimatedStoryPoints int `json:"estimatedStoryPoints" validate:"required" minimum:"0" example:"16"`
	// Story Points estimados por el factor vigente, con hasta 2 decimales (FR-045, RN-08).
	EstimatedHours float64 `json:"estimatedHours" validate:"required" minimum:"0" example:"104"`
}

// ProjectDetail es el estado completo del proyecto (FR-042): la respuesta de la creación y de las
// demás acciones sobre un proyecto. Las fechas van como texto YYYY-MM-DD (research.md sección 4) y
// ActualEndDate y ActiveSprint se serializan como null cuando no existen (FR-043).
type ProjectDetail struct {
	// Identificador propio, estable e inmutable, distinto del nombre (FR-009).
	ID string `json:"id" validate:"required" format:"uuid" example:"5b6c7d8e-9f01-4a23-b456-7890abcdef12"`
	// Nombre del proyecto con la capitalización con que se escribió.
	Name string `json:"name" validate:"required" example:"Portal de Clientes"`
	// Descripción del proyecto, o null si no tiene.
	Description *string `json:"description" extensions:"x-nullable" example:"Rediseño del portal de autogestión."`
	// Estado del proyecto en su ciclo de vida (FR-036).
	Status domain.ProjectStatus `json:"status" validate:"required" enums:"planned,in_progress,finished" example:"planned"`
	// Fecha de inicio (YYYY-MM-DD).
	StartDate string `json:"startDate" validate:"required" format:"date" example:"2026-10-01"`
	// Fecha de finalización prevista (YYYY-MM-DD).
	PlannedEndDate string `json:"plannedEndDate" validate:"required" format:"date" example:"2026-12-20"`
	// Fecha de finalización real, presente solo cuando el estado es finished (FR-039, FR-040).
	ActualEndDate *string `json:"actualEndDate" format:"date" extensions:"x-nullable" example:"2026-12-18"`
	// Factor vigente del proyecto, con hasta 2 decimales (FR-006).
	HoursPerStoryPoint float64 `json:"hoursPerStoryPoint" validate:"required" example:"6.5"`
	// Verdadero cuando el estado es finished: admite consulta y rechaza la administración (FR-018).
	ReadOnly bool `json:"readOnly" validate:"required" example:"false"`
	// Propietario del proyecto.
	Owner OwnerRef `json:"owner" validate:"required"`
	// Integrantes del proyecto; nunca está vacía porque el propietario siempre es integrante (RN-01).
	Members []ProjectMemberView `json:"members" validate:"required,min=1"`
	// Indicación explícita de si hay sprint activo (FR-043).
	HasActiveSprint bool `json:"hasActiveSprint" validate:"required" example:"false"`
	// Sprint activo del proyecto, o null cuando no hay ninguno (FR-043).
	ActiveSprint *ActiveSprintRef `json:"activeSprint" extensions:"x-nullable"`
	// Resumen del backlog del proyecto.
	Backlog BacklogSummary `json:"backlog" validate:"required"`
}

// ProjectListItem es un elemento del listado de proyectos propios (FR-032). Las fechas van como
// texto YYYY-MM-DD (research.md sección 4).
type ProjectListItem struct {
	// Identificador del proyecto.
	ID string `json:"id" validate:"required" format:"uuid" example:"9b2f1c0a-5d3e-4a71-8c62-0f4e7a1b9d35"`
	// Nombre del proyecto.
	Name string `json:"name" validate:"required" example:"Portal de Clientes"`
	// Estado del proyecto en su ciclo de vida (FR-036).
	Status domain.ProjectStatus `json:"status" validate:"required" enums:"planned,in_progress,finished" example:"in_progress"`
	// Fecha de inicio (YYYY-MM-DD).
	StartDate string `json:"startDate" validate:"required" format:"date" example:"2026-10-01"`
	// Fecha de finalización prevista (YYYY-MM-DD).
	PlannedEndDate string `json:"plannedEndDate" validate:"required" format:"date" example:"2026-12-20"`
	// Cantidad de integrantes del proyecto. Siempre 1 o más (FR-007, RN-01).
	MemberCount int `json:"memberCount" validate:"required" minimum:"1" example:"3"`
	// Si quien consulta es el propietario del proyecto.
	IsOwner bool `json:"isOwner" validate:"required" example:"true"`
}

// ProjectListResponse es la respuesta del listado de proyectos (FR-031 a FR-035). Projects se
// serializa siempre como lista, vacía si no hay proyectos, y Message es null cuando hay al menos uno.
type ProjectListResponse struct {
	// Proyectos de los que quien consulta es integrante, ordenados por fecha de inicio descendente y,
	// a igual fecha, nombre ascendente (FR-035).
	Projects []ProjectListItem `json:"projects" validate:"required"`
	// Indicación en español cuando la lista está vacía, para que no se lea como un error (FR-034); es
	// null cuando hay al menos un proyecto.
	Message *string `json:"message" extensions:"x-nullable" example:"Todavía no participás en ningún proyecto."`
}

// newProjectMemberView arma ProjectMemberView a partir del integrante que lista el servicio y es el
// único lugar que decide la visibilidad del correo (FR-030): solo se incluye cuando quien consulta
// es el propietario del proyecto; para cualquier otro integrante queda vacío y omitempty lo quita
// del JSON. La fecha de incorporación se serializa en UTC con formato RFC 3339.
func newProjectMemberView(m domain.MemberView, viewerIsOwner bool) ProjectMemberView {
	view := ProjectMemberView{
		UserID:   m.UserID.String(),
		FullName: m.FullName,
		IsOwner:  m.IsOwner,
		JoinedAt: m.JoinedAt.UTC().Format(time.RFC3339),
	}
	if viewerIsOwner {
		view.Email = m.Email
	}
	return view
}

// hoursFromHundredths convierte un valor en centésimas de hora al número que expone la API. Es la
// única división por 100 de todo el cálculo y ocurre al serializar (research.md sección 3): el
// cociente de dos enteros exactos es el double más cercano al valor de 2 decimales, y encoding/json
// lo escribe con la representación más corta, así que 650 sale como 6.5 y 10400 como 104.
func hoursFromHundredths(hundredths int) float64 {
	return float64(hundredths) / 100
}
