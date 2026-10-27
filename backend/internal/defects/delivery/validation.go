package delivery

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
)

// maxDescriptionLength es la longitud máxima de la descripción, en runas contadas tras recortar los
// espacios extremos (RN-06).
const maxDescriptionLength = 2000

// Mensajes de validación en dirección a la persona, declarados en un único lugar.
const (
	msgDescriptionRequired = "La descripción es obligatoria."
	msgDescriptionTooLong  = "La descripción admite como máximo 2000 caracteres."
	msgSeverityInvalid     = "La severidad debe ser Crítica, Alta, Media o Baja."
	msgStoryRequired       = "La historia relacionada es obligatoria."
	msgSprintRequired      = "El sprint de detección es obligatorio."
	msgIDInvalid           = "El valor no es un identificador válido."

	msgStatusInvalid                = "El estado debe ser Abierto, En progreso, Resuelto o Descartado."
	msgResolutionSprintRequired     = "El sprint de resolución es obligatorio para resolver."
	msgResolutionSprintOnlyResolved = "El sprint de resolución solo se indica al resolver."
)

// ValidateDefectInput valida el cuerpo del alta y devuelve los datos normalizados junto con un
// FieldError por cada campo inválido, todos a la vez (Principio VII). Si hay algún campo inválido,
// se devuelve un DefectInput vacío.
func ValidateDefectInput(req DefectInput) (domain.DefectInput, []authdelivery.FieldError) {
	fields := []authdelivery.FieldError{}
	addField := func(field, message string) {
		fields = append(fields, authdelivery.FieldError{Field: field, Message: message})
	}

	// Descripción recortada, de 1 a 2000 runas (RN-06).
	description := strings.TrimSpace(req.Description)
	switch {
	case description == "":
		addField("description", msgDescriptionRequired)
	case utf8.RuneCountInString(description) > maxDescriptionLength:
		addField("description", msgDescriptionTooLong)
	}

	// Severidad dentro de las cuatro admitidas (RN-04).
	severity := domain.DefectSeverity(req.Severity)
	if !severity.Valid() {
		addField("severity", msgSeverityInvalid)
	}

	// Historia y sprint de detección: obligatorios y con forma de UUID (FR-016).
	storyID, _ := parseRequiredID(req.StoryID, "storyId", msgStoryRequired, addField)
	detectedSprintID, _ := parseRequiredID(req.DetectedSprintID, "detectedSprintId", msgSprintRequired, addField)

	if len(fields) > 0 {
		return domain.DefectInput{}, fields
	}
	return domain.DefectInput{
		StoryID:          storyID,
		DetectedSprintID: detectedSprintID,
		Description:      description,
		Severity:         severity,
	}, nil
}

// parseRequiredID convierte un identificador obligatorio: el texto vacío informa que falta y un UUID
// mal formado informa que no es válido.
func parseRequiredID(text, field, requiredMessage string, addField func(field, message string)) (uuid.UUID, bool) {
	if text == "" {
		addField(field, requiredMessage)
		return uuid.Nil, false
	}
	id, err := uuid.Parse(text)
	if err != nil {
		addField(field, msgIDInvalid)
		return uuid.Nil, false
	}
	return id, true
}

// ValidateStatusChange valida el cuerpo del cambio de estado y devuelve el cambio normalizado junto
// con un FieldError por cada campo inválido (FR-023, Principio VII).
func ValidateStatusChange(req DefectStatusChange) (domain.StatusChange, []authdelivery.FieldError) {
	fields := []authdelivery.FieldError{}
	addField := func(field, message string) {
		fields = append(fields, authdelivery.FieldError{Field: field, Message: message})
	}

	// Estado destino dentro de los cuatro admitidos (RN-11).
	to := domain.DefectStatus(req.Status)
	if !to.Valid() {
		addField("status", msgStatusInvalid)
		return domain.StatusChange{}, fields
	}

	// Sprint de resolución: obligatorio y con forma de UUID al resolver, prohibido en cualquier otro
	// destino (FR-023).
	var resolvedSprintID *uuid.UUID
	switch {
	case to == domain.DefectStatusResolved:
		if id, ok := parseRequiredID(req.ResolvedSprintID, "resolvedSprintId", msgResolutionSprintRequired, addField); ok {
			resolvedSprintID = &id
		}
	case req.ResolvedSprintID != "":
		addField("resolvedSprintId", msgResolutionSprintOnlyResolved)
	}

	if len(fields) > 0 {
		return domain.StatusChange{}, fields
	}
	return domain.StatusChange{To: to, ResolvedSprintID: resolvedSprintID}, nil
}

// ParseDefectFilter convierte los parámetros de consulta del listado en un filtro del dominio
// (firma mínima: la lógica pertenece a T041).
func ParseDefectFilter(statuses, severities []string, storyID, sprintID string) (domain.DefectFilter, []authdelivery.FieldError) {
	return domain.DefectFilter{}, nil
}
