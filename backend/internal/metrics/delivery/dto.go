package delivery

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

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
	var closedAt *string
	if metrics.ClosedAt != nil {
		formatted := metrics.ClosedAt.UTC().Format(time.RFC3339)
		closedAt = &formatted
	}

	// La lista nunca es nula: sin advertencias se serializa como [].
	items := make([]Warning, 0, len(warnings))
	for _, warning := range warnings {
		items = append(items, newWarning(warning))
	}

	return SprintMetrics{
		Sprint: SprintRef{
			ID:        metrics.ID.String(),
			Name:      metrics.Name,
			Status:    string(metrics.Status),
			StartDate: metrics.StartDate.Format(time.DateOnly),
			EndDate:   metrics.EndDate.Format(time.DateOnly),
			ClosedAt:  closedAt,
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
		Warnings:               items,
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
