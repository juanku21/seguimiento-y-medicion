package delivery

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/httpserver"
)

// Mensajes en español de cada código de advertencia (FR-038). Viven en este único lugar: el dominio
// solo conoce el código y los datos, y la capa de entrega decide el texto que lee la persona. El de
// historias del backlog sin estimar lleva la cantidad y se completa con fmt.Sprintf.
const (
	msgUnestimatedBacklogStories = "Hay %d historias del backlog sin estimar; las cifras de planificación pueden quedar cortas."
	msgUnestimatedStoryInSprint  = "Una historia del sprint no tiene Story Points; se contó como 0."
	msgMissingFrozenFactor       = "El sprint cerrado no tiene factor congelado; se usó el factor vigente del proyecto."
	msgMissingCloseTime          = "El sprint cerrado no tiene fecha de cierre; se consideró vencido el plazo de gracia."
)

// Metric es el valor de una métrica con su estado (contracts/openapi.yaml, Metric). Value es un
// número JSON y es null si y solo si Status no es available.
type Metric struct {
	// Estado del valor: available, not_calculable o not_applicable.
	Status string `json:"status" validate:"required" enums:"available,not_calculable,not_applicable" example:"available"`
	// Story Points y conteos como entero; horas y porcentajes con exactamente 2 decimales.
	Value json.RawMessage `json:"value" validate:"required" swaggertype:"number" extensions:"x-nullable" example:"126.00"`
}

// SprintMetric es una métrica de un sprint: agrega la finalidad, que es null en un sprint Planificado.
type SprintMetric struct {
	Metric
	// Finalidad del valor: definitive, provisional o partial; null en un sprint Planificado.
	Finality *string `json:"finality" validate:"required" enums:"definitive,provisional,partial" extensions:"x-nullable" example:"definitive"`
}

// SprintRef identifica el sprint al que corresponden las métricas.
type SprintRef struct {
	// Identificador del sprint.
	ID string `json:"id" validate:"required" format:"uuid" example:"0c6e3f1a-91b2-4d7e-a5c8-2f4b6d8e1a30"`
	// Nombre del sprint.
	Name string `json:"name" validate:"required" example:"Sprint 1"`
	// Estado del sprint.
	Status string `json:"status" validate:"required" enums:"planned,active,closed" example:"closed"`
	// Fecha de inicio (AAAA-MM-DD).
	StartDate string `json:"startDate" validate:"required" format:"date" example:"2026-09-01"`
	// Fecha de fin (AAAA-MM-DD).
	EndDate string `json:"endDate" validate:"required" format:"date" example:"2026-09-12"`
	// Momento real de cierre; null si el sprint no está Cerrado.
	ClosedAt *string `json:"closedAt" validate:"required" format:"date-time" extensions:"x-nullable" example:"2026-09-12T18:30:00Z"`
}

// Warning es una advertencia de inconsistencia o de planificación (contracts/openapi.yaml, Warning).
type Warning struct {
	// Clase de la advertencia: planning o data_inconsistency.
	Kind string `json:"kind" validate:"required" enums:"planning,data_inconsistency" example:"data_inconsistency"`
	// Código del motivo concreto.
	Code string `json:"code" validate:"required" enums:"unestimated_backlog_stories,unestimated_story_in_sprint,missing_frozen_factor,missing_close_time" example:"missing_frozen_factor"`
	// Texto en español para la persona.
	Message string `json:"message" validate:"required" example:"El sprint cerrado no tiene factor congelado; se usó el factor vigente del proyecto."`
	// Sprint afectado; presente en las advertencias de inconsistencia.
	SprintID *string `json:"sprintId,omitempty" format:"uuid" example:"0c6e3f1a-91b2-4d7e-a5c8-2f4b6d8e1a30"`
	// Historia afectada; solo en unestimated_story_in_sprint.
	StoryID *string `json:"storyId,omitempty" format:"uuid" example:"1d7f4a2b-a2c3-4e8f-b6d9-3a5c7e9f2b41"`
	// Cantidad de historias; solo en unestimated_backlog_stories.
	Count *int `json:"count,omitempty" minimum:"1" example:"6"`
}

// SprintMetrics es la respuesta de las métricas de un sprint: el sprint, sus nueve métricas y las
// advertencias de inconsistencia de sus datos.
type SprintMetrics struct {
	Sprint SprintRef `json:"sprint" validate:"required"`
	// Story Points planificados.
	PlannedStoryPoints SprintMetric `json:"plannedStoryPoints" validate:"required"`
	// Story Points completados.
	CompletedStoryPoints SprintMetric `json:"completedStoryPoints" validate:"required"`
	// Horas estimadas (Story Points planificados por el factor).
	EstimatedHours SprintMetric `json:"estimatedHours" validate:"required"`
	// Horas reales registradas en el sprint.
	ActualHours SprintMetric `json:"actualHours" validate:"required"`
	// Horas reales menos horas estimadas; positiva indica subestimación (RN-08).
	DeviationHours SprintMetric `json:"deviationHours" validate:"required"`
	// Desviación sobre las horas estimadas por 100, con el mismo signo; not_calculable si las estimadas son 0.
	DeviationPercent SprintMetric `json:"deviationPercent" validate:"required"`
	// Historias completadas sobre planificadas por 100, por cantidad de historias (RN-09); not_calculable sin historias.
	StoryCompletionPercent SprintMetric `json:"storyCompletionPercent" validate:"required"`
	// Defectos detectados en el sprint.
	DefectsDetected SprintMetric `json:"defectsDetected" validate:"required"`
	// Defectos resueltos en el sprint.
	DefectsResolved SprintMetric `json:"defectsResolved" validate:"required"`
	// Advertencias de inconsistencia de datos del sprint; nunca incluye la de backlog sin estimar (RN-26).
	Warnings []Warning `json:"warnings" validate:"required"`
}

// metricUnit distingue cómo se serializa el valor de una métrica: entero o centésimas con 2 decimales.
type metricUnit int

const (
	// unitCount serializa el valor como entero (Story Points y conteos de defectos).
	unitCount metricUnit = iota
	// unitHundredths serializa el valor con exactamente 2 decimales (horas y porcentajes).
	unitHundredths
)

// newMetric traduce una métrica del dominio; el valor solo se serializa con status available y en
// cualquier otro caso queda null.
func newMetric(metric domain.Metric, unit metricUnit) Metric {
	dto := Metric{Status: string(metric.Status), Value: json.RawMessage("null")}
	if metric.Status != domain.ValueAvailable {
		return dto
	}
	if unit == unitHundredths {
		// Hundredths.MarshalJSON nunca devuelve error; solo se conserva el valor ante un éxito.
		if raw, err := httpserver.Hundredths(metric.Value).MarshalJSON(); err == nil {
			dto.Value = raw
		}
		return dto
	}
	dto.Value = json.RawMessage(strconv.Itoa(metric.Value))
	return dto
}

// newSprintMetric traduce una métrica de sprint con su finalidad anulable.
func newSprintMetric(metric domain.Metric, unit metricUnit) SprintMetric {
	dto := SprintMetric{Metric: newMetric(metric, unit)}
	if metric.Finality != nil {
		finality := string(*metric.Finality)
		dto.Finality = &finality
	}
	return dto
}

// newSprintMetrics traduce el resultado del cálculo de un sprint y sus advertencias al contrato.
func newSprintMetrics(metrics domain.SprintMetrics, warnings []domain.Warning) SprintMetrics {
	return SprintMetrics{
		Sprint: SprintRef{
			ID:        metrics.ID.String(),
			Name:      metrics.Name,
			Status:    string(metrics.Status),
			StartDate: metrics.StartDate.Format(time.DateOnly),
			EndDate:   metrics.EndDate.Format(time.DateOnly),
			ClosedAt:  formatClosedAt(metrics.ClosedAt),
		},
		PlannedStoryPoints:     newSprintMetric(metrics.PlannedStoryPoints, unitCount),
		CompletedStoryPoints:   newSprintMetric(metrics.CompletedStoryPoints, unitCount),
		EstimatedHours:         newSprintMetric(metrics.EstimatedHours, unitHundredths),
		ActualHours:            newSprintMetric(metrics.ActualHours, unitHundredths),
		DeviationHours:         newSprintMetric(metrics.DeviationHours, unitHundredths),
		DeviationPercent:       newSprintMetric(metrics.DeviationPercent, unitHundredths),
		StoryCompletionPercent: newSprintMetric(metrics.StoryCompletionPercent, unitHundredths),
		DefectsDetected:        newSprintMetric(metrics.DefectsDetected, unitCount),
		DefectsResolved:        newSprintMetric(metrics.DefectsResolved, unitCount),
		Warnings:               newWarnings(warnings),
	}
}

// newWarnings traduce una lista de advertencias; nunca es nula: sin advertencias se serializa como [].
func newWarnings(warnings []domain.Warning) []Warning {
	items := make([]Warning, 0, len(warnings))
	for _, warning := range warnings {
		items = append(items, newWarning(warning))
	}
	return items
}

// VelocityPoint es la velocidad de un sprint Cerrado (contracts/openapi.yaml, VelocityPoint).
type VelocityPoint struct {
	// Identificador del sprint Cerrado.
	SprintID string `json:"sprintId" validate:"required" format:"uuid" example:"0c6e3f1a-91b2-4d7e-a5c8-2f4b6d8e1a30"`
	// Nombre del sprint.
	SprintName string `json:"sprintName" validate:"required" example:"Sprint 1"`
	// Momento real de cierre; null si el sprint cerrado no lo registra.
	ClosedAt *string `json:"closedAt" validate:"required" format:"date-time" extensions:"x-nullable" example:"2026-09-12T18:30:00Z"`
	// Story Points completados en el sprint.
	CompletedStoryPoints int `json:"completedStoryPoints" validate:"required" minimum:"0" example:"13"`
}

// Velocity es la velocidad del equipo; nunca lleva finalidad porque solo mira sprints Cerrados.
type Velocity struct {
	// Promedio de Story Points completados de los últimos 3 sprints Cerrados, con 2 decimales.
	Value Metric `json:"value" validate:"required"`
	// Cantidad de sprints Cerrados que entraron en el promedio.
	SprintsConsidered int `json:"sprintsConsidered" validate:"required" minimum:"0" maximum:"3" example:"3"`
	// Story Points completados de cada sprint Cerrado, en orden cronológico, sin promediar ni recortar.
	Series []VelocityPoint `json:"series" validate:"required"`
}

// Cumulative es la suma de los sprints Cerrados y del Activo (contracts/openapi.yaml, Cumulative). La
// marca de finalidad califica al bloque entero y no a cada métrica (FR-023).
type Cumulative struct {
	// Finalidad del bloque: partial si incluye el sprint Activo; definitive si solo hay Cerrados.
	Finality string `json:"finality" validate:"required" enums:"definitive,partial" example:"partial"`
	// Indica si la suma incluye el sprint Activo.
	IncludesActiveSprint bool `json:"includesActiveSprint" validate:"required" example:"true"`
	// Story Points planificados acumulados.
	PlannedStoryPoints Metric `json:"plannedStoryPoints" validate:"required"`
	// Story Points completados acumulados.
	CompletedStoryPoints Metric `json:"completedStoryPoints" validate:"required"`
	// Horas estimadas acumuladas.
	EstimatedHours Metric `json:"estimatedHours" validate:"required"`
	// Horas reales acumuladas.
	ActualHours Metric `json:"actualHours" validate:"required"`
	// Horas reales menos horas estimadas acumuladas (RN-08).
	DeviationHours Metric `json:"deviationHours" validate:"required"`
	// Desviación sobre las horas estimadas acumuladas por 100; sale de los totales (RN-11).
	DeviationPercent Metric `json:"deviationPercent" validate:"required"`
	// Historias completadas sobre planificadas acumuladas por 100 (RN-09, RN-11).
	StoryCompletionPercent Metric `json:"storyCompletionPercent" validate:"required"`
}

// ProjectMetricsResponse es la respuesta de las métricas del proyecto: la velocidad, los acumulados,
// el esfuerzo fuera de los sprints Cerrados y Activo, el contador del backlog sin estimar y las
// advertencias.
type ProjectMetricsResponse struct {
	// Identificador del proyecto.
	ProjectID string `json:"projectId" validate:"required" format:"uuid" example:"6b1f0d4e-2a7c-4e19-9d3b-8f5a2c1e7d40"`
	// Velocidad del equipo y su serie.
	Velocity Velocity `json:"velocity" validate:"required"`
	// Métricas acumuladas de los sprints Cerrados y el Activo.
	Cumulative Cumulative `json:"cumulative" validate:"required"`
	// Horas de registros sin sprint asociado, con 2 decimales; 0 incluido (FR-029).
	EffortOutsideSprintsHours httpserver.Hundredths `json:"effortOutsideSprintsHours" validate:"required" swaggertype:"number" example:"18.00"`
	// Horas de registros anclados a sprints Planificados, con 2 decimales; 0 incluido.
	EffortInPlannedSprintsHours httpserver.Hundredths `json:"effortInPlannedSprintsHours" validate:"required" swaggertype:"number" example:"0.00"`
	// Historias del proyecto sin estimar (FR-036).
	UnestimatedBacklogStories int `json:"unestimatedBacklogStories" validate:"required" minimum:"0" example:"6"`
	// Advertencias de planificación y de inconsistencia de datos.
	Warnings []Warning `json:"warnings" validate:"required"`
}

// SeriesPoint es el valor de una métrica en un sprint; plano: identifica su sprint y conserva la
// finalidad de la métrica en ese sprint.
type SeriesPoint struct {
	// Identificador del sprint.
	SprintID string `json:"sprintId" validate:"required" format:"uuid" example:"0c6e3f1a-91b2-4d7e-a5c8-2f4b6d8e1a30"`
	// Nombre del sprint.
	SprintName string `json:"sprintName" validate:"required" example:"Sprint 1"`
	// Estado del sprint: active o closed.
	SprintStatus string `json:"sprintStatus" validate:"required" enums:"active,closed" example:"closed"`
	Metric
	// Finalidad del valor: definitive, provisional o partial.
	Finality *string `json:"finality" validate:"required" enums:"definitive,provisional,partial" extensions:"x-nullable" example:"definitive"`
}

// ProjectSeriesResponse son las seis series por sprint del proyecto, con los mismos sprints en el
// mismo orden cronológico.
type ProjectSeriesResponse struct {
	// Story Points planificados por sprint.
	PlannedStoryPoints []SeriesPoint `json:"plannedStoryPoints" validate:"required"`
	// Story Points completados por sprint (velocidad por sprint).
	CompletedStoryPoints []SeriesPoint `json:"completedStoryPoints" validate:"required"`
	// Horas estimadas por sprint.
	EstimatedHours []SeriesPoint `json:"estimatedHours" validate:"required"`
	// Horas reales por sprint.
	ActualHours []SeriesPoint `json:"actualHours" validate:"required"`
	// Defectos detectados por sprint.
	DefectsDetected []SeriesPoint `json:"defectsDetected" validate:"required"`
	// Defectos resueltos por sprint.
	DefectsResolved []SeriesPoint `json:"defectsResolved" validate:"required"`
	// Advertencias de inconsistencia de datos de los sprints incluidos.
	Warnings []Warning `json:"warnings" validate:"required"`
}

// formatClosedAt formatea el momento de cierre en RFC 3339 UTC; es nil si el sprint no lo registra.
func formatClosedAt(closedAt *time.Time) *string {
	if closedAt == nil {
		return nil
	}
	formatted := closedAt.UTC().Format(time.RFC3339)
	return &formatted
}

// newVelocityPoints traduce la serie de velocidad; la lista nunca es nula: sin sprints Cerrados se
// serializa como [].
func newVelocityPoints(points []domain.VelocityPoint) []VelocityPoint {
	items := make([]VelocityPoint, 0, len(points))
	for _, point := range points {
		items = append(items, VelocityPoint{
			SprintID:             point.SprintID.String(),
			SprintName:           point.SprintName,
			ClosedAt:             formatClosedAt(point.ClosedAt),
			CompletedStoryPoints: point.CompletedStoryPoints,
		})
	}
	return items
}

// newProjectMetrics traduce el resultado del cálculo del proyecto al contrato.
func newProjectMetrics(projectID uuid.UUID, metrics domain.ProjectMetrics) ProjectMetricsResponse {
	return ProjectMetricsResponse{
		ProjectID: projectID.String(),
		Velocity: Velocity{
			Value:             newMetric(metrics.Velocity.Value, unitHundredths),
			SprintsConsidered: metrics.Velocity.SprintsConsidered,
			Series:            newVelocityPoints(metrics.Velocity.Series),
		},
		Cumulative:                  newCumulative(metrics.Cumulative),
		EffortOutsideSprintsHours:   httpserver.Hundredths(metrics.EffortOutsideSprintsHundredths),
		EffortInPlannedSprintsHours: httpserver.Hundredths(metrics.EffortInPlannedSprintsHundredths),
		UnestimatedBacklogStories:   metrics.UnestimatedBacklogStories,
		Warnings:                    newWarnings(metrics.Warnings),
	}
}

// newCumulative traduce el bloque acumulado: enteros para Story Points y 2 decimales para horas y
// porcentajes.
func newCumulative(cumulative domain.Cumulative) Cumulative {
	return Cumulative{
		Finality:               string(cumulative.Finality),
		IncludesActiveSprint:   cumulative.IncludesActiveSprint,
		PlannedStoryPoints:     newMetric(cumulative.PlannedStoryPoints, unitCount),
		CompletedStoryPoints:   newMetric(cumulative.CompletedStoryPoints, unitCount),
		EstimatedHours:         newMetric(cumulative.EstimatedHours, unitHundredths),
		ActualHours:            newMetric(cumulative.ActualHours, unitHundredths),
		DeviationHours:         newMetric(cumulative.DeviationHours, unitHundredths),
		DeviationPercent:       newMetric(cumulative.DeviationPercent, unitHundredths),
		StoryCompletionPercent: newMetric(cumulative.StoryCompletionPercent, unitHundredths),
	}
}

// newSeriesPoints traduce una serie del dominio; la lista nunca es nula.
func newSeriesPoints(points []domain.SeriesPoint, unit metricUnit) []SeriesPoint {
	items := make([]SeriesPoint, 0, len(points))
	for _, point := range points {
		sprintMetric := newSprintMetric(point.Value, unit)
		items = append(items, SeriesPoint{
			SprintID:     point.SprintID.String(),
			SprintName:   point.SprintName,
			SprintStatus: string(point.SprintStatus),
			Metric:       sprintMetric.Metric,
			Finality:     sprintMetric.Finality,
		})
	}
	return items
}

// newProjectSeries traduce las seis series y las advertencias al contrato.
func newProjectSeries(series domain.ProjectSeries, warnings []domain.Warning) ProjectSeriesResponse {
	return ProjectSeriesResponse{
		PlannedStoryPoints:   newSeriesPoints(series.PlannedStoryPoints, unitCount),
		CompletedStoryPoints: newSeriesPoints(series.CompletedStoryPoints, unitCount),
		EstimatedHours:       newSeriesPoints(series.EstimatedHours, unitHundredths),
		ActualHours:          newSeriesPoints(series.ActualHours, unitHundredths),
		DefectsDetected:      newSeriesPoints(series.DefectsDetected, unitCount),
		DefectsResolved:      newSeriesPoints(series.DefectsResolved, unitCount),
		Warnings:             newWarnings(warnings),
	}
}

// newWarning traduce una advertencia del dominio y le asigna el mensaje en español de su código.
func newWarning(warning domain.Warning) Warning {
	dto := Warning{
		Kind:    string(warning.Kind),
		Code:    string(warning.Code),
		Message: warningMessage(warning),
		Count:   warning.Count,
	}
	if warning.SprintID != nil {
		sprintID := warning.SprintID.String()
		dto.SprintID = &sprintID
	}
	if warning.StoryID != nil {
		storyID := warning.StoryID.String()
		dto.StoryID = &storyID
	}
	return dto
}

// warningMessage devuelve el texto de una advertencia según su código.
func warningMessage(warning domain.Warning) string {
	switch warning.Code {
	case domain.CodeUnestimatedBacklogStories:
		count := 0
		if warning.Count != nil {
			count = *warning.Count
		}
		return fmt.Sprintf(msgUnestimatedBacklogStories, count)
	case domain.CodeUnestimatedStoryInSprint:
		return msgUnestimatedStoryInSprint
	case domain.CodeMissingFrozenFactor:
		return msgMissingFrozenFactor
	case domain.CodeMissingCloseTime:
		return msgMissingCloseTime
	}
	return ""
}
