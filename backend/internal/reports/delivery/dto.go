package delivery

import (
	"time"

	metricsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/httpserver"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/domain"
)

// ReportSprint es el sprint que el reporte referencia (contracts/openapi.yaml, ReportSprint).
type ReportSprint struct {
	// Identificador del sprint.
	ID string `json:"id" validate:"required" format:"uuid" example:"0c6e3f1a-91b2-4d7e-a5c8-2f4b6d8e1a30"`
	// Nombre del sprint.
	Name string `json:"name" validate:"required" example:"Sprint 3"`
	// Estado del sprint.
	Status string `json:"status" validate:"required" enums:"active,closed" example:"closed"`
	// Fecha de inicio (AAAA-MM-DD).
	StartDate string `json:"startDate" validate:"required" format:"date" example:"2026-09-15"`
	// Fecha de fin (AAAA-MM-DD).
	EndDate string `json:"endDate" validate:"required" format:"date" example:"2026-09-28"`
	// Instante de cierre; null en el sprint Activo.
	ClosedAt *string `json:"closedAt" validate:"required" format:"date-time" extensions:"x-nullable" example:"2026-09-28T18:30:00Z"`
}

// Period es el período que cubre el reporte.
type Period struct {
	// Fecha de inicio (AAAA-MM-DD).
	StartDate string `json:"startDate" validate:"required" format:"date" example:"2026-09-15"`
	// Fecha de fin (AAAA-MM-DD).
	EndDate string `json:"endDate" validate:"required" format:"date" example:"2026-09-28"`
}

// GeneratedBy es la persona que generó el reporte.
type GeneratedBy struct {
	// Identificador del usuario.
	ID string `json:"id" validate:"required" format:"uuid" example:"1d7f4a2b-a2c3-4e8f-b6d9-3a5c7e9f2b41"`
	// Nombre completo del usuario.
	FullName string `json:"fullName" validate:"required" example:"Ana Pérez"`
}

// Header es el encabezado del reporte.
type Header struct {
	// Identificador del proyecto.
	ProjectID string `json:"projectId" validate:"required" format:"uuid" example:"2e8a5b3c-b3d4-4f9a-87ea-4b6d8f0a3c52"`
	// Nombre del proyecto.
	ProjectName string `json:"projectName" validate:"required" example:"Gestión de Compras — Ñandú"`
	// Estado del proyecto.
	ProjectStatus string `json:"projectStatus" validate:"required" enums:"planned,in_progress,finished" example:"in_progress"`
	// Sprint del reporte; null en el alcance project.
	Sprint *ReportSprint `json:"sprint" validate:"required" extensions:"x-nullable"`
	// Período cubierto; null en un proyecto sin sprints reportables.
	Period *Period `json:"period" validate:"required" extensions:"x-nullable"`
	// Instante de la generación, en UTC; el mismo que guarda el registro.
	GeneratedAt string `json:"generatedAt" validate:"required" format:"date-time" example:"2026-10-30T17:32:05Z"`
	// Quien generó el reporte.
	GeneratedBy GeneratedBy `json:"generatedBy" validate:"required"`
}

// StoryLine es una historia dentro de un grupo.
type StoryLine struct {
	// Identificador de la historia.
	StoryID string `json:"storyId" validate:"required" format:"uuid" example:"3f9b6c4d-c4e5-4a0b-98fb-5c7e9a1b4d63"`
	// Título de la historia.
	Title string `json:"title" validate:"required" example:"Alta de clientes"`
	// Prioridad de la historia.
	Priority string `json:"priority" validate:"required" enums:"high,medium,low" example:"high"`
	// Story Points; null si la historia está sin estimar.
	StoryPoints *int `json:"storyPoints" validate:"required" extensions:"x-nullable" example:"5"`
	// Resultado congelado en un sprint Cerrado; estado vigente en el Activo y en las no planificadas.
	Status string `json:"status" validate:"required" enums:"pending,in_progress,completed,not_completed" example:"completed"`
}

// StoryGroup agrupa las historias de un sprint.
type StoryGroup struct {
	// Sprint del grupo; null en el grupo de historias no planificadas.
	Sprint *ReportSprint `json:"sprint" validate:"required" extensions:"x-nullable"`
	// Estado del grupo: complete o empty.
	State string `json:"state" validate:"required" enums:"complete,empty" example:"complete"`
	// Historias del grupo, por prioridad, después título y después identificador.
	Stories []StoryLine `json:"stories" validate:"required"`
}

// StoriesSection es la sección de historias.
type StoriesSection struct {
	// Estado de la sección.
	State string `json:"state" validate:"required" enums:"complete,empty,unavailable" example:"complete"`
	// Grupos de historias; null con unavailable.
	Groups []StoryGroup `json:"groups" validate:"required" extensions:"x-nullable"`
}

// PokerSummary resume la sesión de Planning Poker que acordó el valor.
type PokerSummary struct {
	// Cantidad de rondas de la sesión.
	Rounds int `json:"rounds" validate:"required" minimum:"1" example:"3"`
	// Story Points acordados.
	AgreedStoryPoints int `json:"agreedStoryPoints" validate:"required" example:"8"`
	// Instante en que finalizó la sesión.
	FinishedAt string `json:"finishedAt" validate:"required" format:"date-time" example:"2026-09-16T14:00:00Z"`
}

// EstimationLine es la estimación de una historia.
type EstimationLine struct {
	// Identificador de la historia.
	StoryID string `json:"storyId" validate:"required" format:"uuid" example:"3f9b6c4d-c4e5-4a0b-98fb-5c7e9a1b4d63"`
	// Título de la historia.
	Title string `json:"title" validate:"required" example:"Alta de clientes"`
	// Story Points; null si la historia está sin estimar.
	StoryPoints *int `json:"storyPoints" validate:"required" extensions:"x-nullable" example:"8"`
	// agreed si hubo una sesión Finalizada, no_agreement si hubo sesiones sin ninguna Finalizada y none si no hubo.
	Poker string `json:"poker" validate:"required" enums:"agreed,no_agreement,none" example:"agreed"`
	// Última sesión Finalizada; solo con agreed.
	Session *PokerSummary `json:"session" validate:"required" extensions:"x-nullable"`
}

// EstimationGroup agrupa las estimaciones de un sprint.
type EstimationGroup struct {
	// Sprint del grupo; null en el grupo de historias no planificadas.
	Sprint *ReportSprint `json:"sprint" validate:"required" extensions:"x-nullable"`
	// Estado del grupo: complete o empty.
	State string `json:"state" validate:"required" enums:"complete,empty" example:"complete"`
	// Estimaciones del grupo.
	Items []EstimationLine `json:"items" validate:"required"`
}

// EstimationsSection es la sección de estimaciones.
type EstimationsSection struct {
	// Estado de la sección.
	State string `json:"state" validate:"required" enums:"complete,empty,unavailable" example:"complete"`
	// Los mismos grupos, en el mismo orden, que la sección de historias; null con unavailable.
	Groups []EstimationGroup `json:"groups" validate:"required" extensions:"x-nullable"`
}

// StoryEffort es el esfuerzo de una historia dentro de un sprint.
type StoryEffort struct {
	// Identificador de la historia.
	StoryID string `json:"storyId" validate:"required" format:"uuid" example:"3f9b6c4d-c4e5-4a0b-98fb-5c7e9a1b4d63"`
	// Título de la historia.
	Title string `json:"title" validate:"required" example:"Alta de clientes"`
	// Horas registradas, con 2 decimales.
	Hours httpserver.Hundredths `json:"hours" validate:"required" swaggertype:"number" example:"12.50"`
}

// EffortGroup agrupa el esfuerzo de un sprint.
type EffortGroup struct {
	// Sprint del grupo.
	Sprint ReportSprint `json:"sprint" validate:"required"`
	// Estado del grupo: complete o empty.
	State string `json:"state" validate:"required" enums:"complete,empty" example:"complete"`
	// Esfuerzo por historia.
	Stories []StoryEffort `json:"stories" validate:"required"`
	// Total de horas del grupo, con 2 decimales.
	TotalHours httpserver.Hundredths `json:"totalHours" validate:"required" swaggertype:"number" example:"40.00"`
}

// MemberEffort es el esfuerzo de un integrante en todo el alcance.
type MemberEffort struct {
	// Identificador del usuario.
	UserID string `json:"userId" validate:"required" format:"uuid" example:"1d7f4a2b-a2c3-4e8f-b6d9-3a5c7e9f2b41"`
	// Nombre completo; se conserva aunque la persona ya no sea integrante.
	FullName string `json:"fullName" validate:"required" example:"Ana Pérez"`
	// Horas registradas, con 2 decimales.
	Hours httpserver.Hundredths `json:"hours" validate:"required" swaggertype:"number" example:"18.25"`
}

// EffortSection es la sección de esfuerzo.
type EffortSection struct {
	// Estado de la sección.
	State string `json:"state" validate:"required" enums:"complete,empty,unavailable" example:"complete"`
	// Un grupo por sprint incluido; null con unavailable.
	Groups []EffortGroup `json:"groups" validate:"required" extensions:"x-nullable"`
	// Esfuerzo por integrante en todo el alcance; null con unavailable.
	ByMember []MemberEffort `json:"byMember" validate:"required" extensions:"x-nullable"`
	// Suma de los registros del alcance, con 2 decimales; 0.00 con empty y null con unavailable.
	TotalHours *httpserver.Hundredths `json:"totalHours" validate:"required" swaggertype:"number" extensions:"x-nullable" example:"58.25"`
}

// MetricsSection es la sección de métricas: exactamente uno de Sprint o Project está presente según
// el alcance, con el cuerpo de la 008 sin cambios.
type MetricsSection struct {
	// Estado de la sección.
	State string `json:"state" validate:"required" enums:"complete,empty,unavailable" example:"complete"`
	// Métricas del sprint; null fuera del alcance sprint.
	Sprint *metricsdelivery.SprintMetrics `json:"sprint" validate:"required" extensions:"x-nullable"`
	// Métricas del proyecto; null fuera del alcance project.
	Project *metricsdelivery.ProjectMetricsResponse `json:"project" validate:"required" extensions:"x-nullable"`
}

// SprintName identifica un sprint por su identificador y su nombre.
type SprintName struct {
	// Identificador del sprint.
	ID string `json:"id" validate:"required" format:"uuid" example:"0c6e3f1a-91b2-4d7e-a5c8-2f4b6d8e1a30"`
	// Nombre del sprint.
	Name string `json:"name" validate:"required" example:"Sprint 3"`
}

// DefectStory identifica la historia de un defecto.
type DefectStory struct {
	// Identificador de la historia.
	ID string `json:"id" validate:"required" format:"uuid" example:"3f9b6c4d-c4e5-4a0b-98fb-5c7e9a1b4d63"`
	// Título de la instantánea del sprint del reporte si la historia está en ella; si no, el vigente.
	Title string `json:"title" validate:"required" example:"Alta de clientes"`
}

// DefectLine es un defecto del listado.
type DefectLine struct {
	// Identificador del defecto.
	DefectID string `json:"defectId" validate:"required" format:"uuid" example:"4a0c7d5e-d5f6-4b1c-a90c-6d8fab2c5e74"`
	// Descripción del defecto.
	Description string `json:"description" validate:"required" example:"El formulario no valida el CUIT"`
	// Severidad del defecto.
	Severity string `json:"severity" validate:"required" enums:"critical,high,medium,low" example:"high"`
	// Estado del defecto; los Descartados nunca se listan.
	Status string `json:"status" validate:"required" enums:"open,in_progress,resolved" example:"open"`
	// Historia afectada.
	Story DefectStory `json:"story" validate:"required"`
	// Sprint en que se detectó.
	DetectedSprint SprintName `json:"detectedSprint" validate:"required"`
	// Sprint en que se resolvió; null si no se resolvió.
	ResolvedSprint *SprintName `json:"resolvedSprint" validate:"required" extensions:"x-nullable"`
}

// DefectTotals son los totales de defectos que informa la 008.
type DefectTotals struct {
	// Defectos detectados.
	Detected int `json:"detected" validate:"required" minimum:"0" example:"4"`
	// Defectos resueltos.
	Resolved int `json:"resolved" validate:"required" minimum:"0" example:"2"`
}

// DefectsSection es la sección de defectos.
type DefectsSection struct {
	// Estado de la sección.
	State string `json:"state" validate:"required" enums:"complete,empty,unavailable" example:"complete"`
	// Defectos del alcance; null con unavailable.
	Defects []DefectLine `json:"defects" validate:"required" extensions:"x-nullable"`
	// Totales de la 008; presentes también con empty (en 0) y null con unavailable.
	Totals *DefectTotals `json:"totals" validate:"required" extensions:"x-nullable"`
}

// Report es el reporte consolidado de un sprint o del proyecto (contracts/openapi.yaml, Report).
type Report struct {
	// Alcance del reporte.
	Scope string `json:"scope" validate:"required" enums:"sprint,project" example:"sprint"`
	// true si el reporte incluye el sprint Activo.
	Partial bool `json:"partial" validate:"required" example:"false"`
	// Encabezado.
	Header Header `json:"header" validate:"required"`
	// Sección de historias.
	Stories StoriesSection `json:"stories" validate:"required"`
	// Sección de estimaciones.
	Estimations EstimationsSection `json:"estimations" validate:"required"`
	// Sección de esfuerzo.
	Effort EffortSection `json:"effort" validate:"required"`
	// Sección de métricas.
	Metrics MetricsSection `json:"metrics" validate:"required"`
	// Sección de defectos.
	Defects DefectsSection `json:"defects" validate:"required"`
}

// formatInstant da formato RFC 3339 en UTC a un instante.
func formatInstant(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// newReportSprint traduce la referencia a un sprint; es nula si el origen lo es.
func newReportSprint(sprint *domain.SprintRef) *ReportSprint {
	if sprint == nil {
		return nil
	}
	dto := newReportSprintValue(*sprint)
	return &dto
}

// newReportSprintValue traduce un sprint que siempre está presente.
func newReportSprintValue(sprint domain.SprintRef) ReportSprint {
	dto := ReportSprint{
		ID:        sprint.ID.String(),
		Name:      sprint.Name,
		Status:    string(sprint.Status),
		StartDate: sprint.StartDate.Format(time.DateOnly),
		EndDate:   sprint.EndDate.Format(time.DateOnly),
	}
	if sprint.ClosedAt != nil {
		closedAt := formatInstant(*sprint.ClosedAt)
		dto.ClosedAt = &closedAt
	}
	return dto
}

// newHundredths convierte centésimas enteras al tipo que se serializa con 2 decimales.
func newHundredths(hundredths int) httpserver.Hundredths {
	return httpserver.Hundredths(hundredths)
}

// newHeader traduce el encabezado.
func newHeader(header domain.Header) Header {
	dto := Header{
		ProjectID:     header.Project.ID.String(),
		ProjectName:   header.Project.Name,
		ProjectStatus: string(header.Project.Status),
		Sprint:        newReportSprint(header.Sprint),
		GeneratedAt:   formatInstant(header.GeneratedAt),
		GeneratedBy:   GeneratedBy{ID: header.GeneratedBy.ID.String(), FullName: header.GeneratedBy.FullName},
	}
	if header.Period != nil {
		dto.Period = &Period{
			StartDate: header.Period.StartDate.Format(time.DateOnly),
			EndDate:   header.Period.EndDate.Format(time.DateOnly),
		}
	}
	return dto
}

// newStoriesSection traduce la sección de historias; las listas de un grupo nunca son nulas.
func newStoriesSection(section domain.StoriesSection) StoriesSection {
	dto := StoriesSection{State: string(section.State)}
	if section.Groups == nil {
		return dto
	}
	dto.Groups = make([]StoryGroup, 0, len(section.Groups))
	for _, group := range section.Groups {
		stories := make([]StoryLine, 0, len(group.Stories))
		for _, story := range group.Stories {
			stories = append(stories, StoryLine{
				StoryID:     story.StoryID.String(),
				Title:       story.Title,
				Priority:    string(story.Priority),
				StoryPoints: story.StoryPoints,
				Status:      string(story.Status),
			})
		}
		dto.Groups = append(dto.Groups, StoryGroup{Sprint: newReportSprint(group.Sprint), State: string(group.State), Stories: stories})
	}
	return dto
}

// newEstimationsSection traduce la sección de estimaciones; las listas de un grupo nunca son nulas.
func newEstimationsSection(section domain.EstimationsSection) EstimationsSection {
	dto := EstimationsSection{State: string(section.State)}
	if section.Groups == nil {
		return dto
	}
	dto.Groups = make([]EstimationGroup, 0, len(section.Groups))
	for _, group := range section.Groups {
		items := make([]EstimationLine, 0, len(group.Items))
		for _, item := range group.Items {
			line := EstimationLine{
				StoryID:     item.StoryID.String(),
				Title:       item.Title,
				StoryPoints: item.StoryPoints,
				Poker:       string(item.Poker),
			}
			if item.Session != nil {
				line.Session = &PokerSummary{
					Rounds:            item.Session.Rounds,
					AgreedStoryPoints: item.Session.AgreedStoryPoints,
					FinishedAt:        formatInstant(item.Session.FinishedAt),
				}
			}
			items = append(items, line)
		}
		dto.Groups = append(dto.Groups, EstimationGroup{Sprint: newReportSprint(group.Sprint), State: string(group.State), Items: items})
	}
	return dto
}

// newEffortSection traduce la sección de esfuerzo; el total es nulo solo con unavailable.
func newEffortSection(section domain.EffortSection) EffortSection {
	dto := EffortSection{State: string(section.State)}
	if section.State == domain.SectionUnavailable {
		return dto
	}
	total := newHundredths(section.TotalHundredths)
	dto.TotalHours = &total
	if section.Groups != nil {
		dto.Groups = make([]EffortGroup, 0, len(section.Groups))
		for _, group := range section.Groups {
			stories := make([]StoryEffort, 0, len(group.Stories))
			for _, story := range group.Stories {
				stories = append(stories, StoryEffort{StoryID: story.StoryID.String(), Title: story.Title, Hours: newHundredths(story.Hundredths)})
			}
			dto.Groups = append(dto.Groups, EffortGroup{
				Sprint:     newReportSprintValue(group.Sprint),
				State:      string(group.State),
				Stories:    stories,
				TotalHours: newHundredths(group.TotalHundredths),
			})
		}
	}
	if section.ByMember != nil {
		dto.ByMember = make([]MemberEffort, 0, len(section.ByMember))
		for _, member := range section.ByMember {
			dto.ByMember = append(dto.ByMember, MemberEffort{UserID: member.UserID.String(), FullName: member.FullName, Hours: newHundredths(member.Hundredths)})
		}
	}
	return dto
}

// newMetricsSection traduce la sección de métricas reutilizando los cuerpos de la 008.
func newMetricsSection(section domain.MetricsSection, header domain.Header) MetricsSection {
	dto := MetricsSection{State: string(section.State)}
	if section.Sprint != nil {
		sprint := metricsdelivery.NewSprintMetrics(*section.Sprint, section.SprintWarnings)
		dto.Sprint = &sprint
	}
	if section.Project != nil {
		project := metricsdelivery.NewProjectMetrics(header.Project.ID, *section.Project)
		dto.Project = &project
	}
	return dto
}

// newDefectsSection traduce la sección de defectos.
func newDefectsSection(section domain.DefectsSection) DefectsSection {
	dto := DefectsSection{State: string(section.State)}
	if section.Defects != nil {
		dto.Defects = make([]DefectLine, 0, len(section.Defects))
		for _, defect := range section.Defects {
			line := DefectLine{
				DefectID:       defect.DefectID.String(),
				Description:    defect.Description,
				Severity:       string(defect.Severity),
				Status:         string(defect.Status),
				Story:          DefectStory{ID: defect.Story.ID.String(), Title: defect.Story.Title},
				DetectedSprint: SprintName{ID: defect.DetectedSprint.ID.String(), Name: defect.DetectedSprint.Name},
			}
			if defect.ResolvedSprint != nil {
				line.ResolvedSprint = &SprintName{ID: defect.ResolvedSprint.ID.String(), Name: defect.ResolvedSprint.Name}
			}
			dto.Defects = append(dto.Defects, line)
		}
	}
	if section.Totals != nil {
		dto.Totals = &DefectTotals{Detected: section.Totals.Detected, Resolved: section.Totals.Resolved}
	}
	return dto
}

// NewReport traduce el reporte del dominio a la forma del contrato.
func NewReport(report domain.Report) Report {
	return Report{
		Scope:       string(report.Scope),
		Partial:     report.Partial,
		Header:      newHeader(report.Header),
		Stories:     newStoriesSection(report.Stories),
		Estimations: newEstimationsSection(report.Estimations),
		Effort:      newEffortSection(report.Effort),
		Metrics:     newMetricsSection(report.Metrics, report.Header),
		Defects:     newDefectsSection(report.Defects),
	}
}
