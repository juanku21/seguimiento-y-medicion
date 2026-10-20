package delivery

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// Longitudes máximas de la entrada de historias, en runas contadas tras recortar los espacios
// extremos y declaradas en un único lugar (RN-03, RN-04, RN-08).
const (
	maxTitleLength       = 150
	maxDescriptionLength = 2000
	maxCriterionLength   = 500
)

// Mensajes de validación en español dirigidos a la persona (data-model.md, reglas de validación).
// Los límites se toman de las constantes y la escala de Story Points se deriva de
// domain.StoryPointsScale para no repetirlos en el texto.
var (
	msgTitleRequired      = "El título es obligatorio."
	msgTitleTooLong       = fmt.Sprintf("El título admite como máximo %d caracteres.", maxTitleLength)
	msgDescriptionTooLong = fmt.Sprintf("La descripción admite como máximo %d caracteres.",
		maxDescriptionLength)
	msgInvalidPriority    = "Los valores admitidos son Alta, Media y Baja."
	msgInvalidStoryPoints = "Los valores admitidos son " + storyPointsScaleText() + "."
	msgCriterionRequired  = "Cada criterio de aceptación debe tener contenido."
	msgCriterionTooLong   = fmt.Sprintf("Cada criterio de aceptación admite como máximo %d caracteres.",
		maxCriterionLength)
)

// ValidatedStory son los datos de una historia ya validados y normalizados: título y criterios
// recortados, descripción nula si queda vacía y Story Points nulos si la historia está sin estimar.
type ValidatedStory struct {
	Title              string
	Description        *string
	Priority           domain.StoryPriority
	StoryPoints        *int
	AcceptanceCriteria []string
}

// ValidateStoryInput aplica las reglas RN-03 a RN-08 a la entrada de la creación y devuelve los
// datos normalizados junto con un FieldError por cada campo inválido, todos a la vez (FR-013). Si
// hay algún campo inválido, los datos devueltos no deben usarse.
func ValidateStoryInput(req StoryInput) (ValidatedStory, []authdelivery.FieldError) {
	fields := []authdelivery.FieldError{}
	addField := func(field, message string) {
		fields = append(fields, authdelivery.FieldError{Field: field, Message: message})
	}

	// Título obligatorio, recortado y con longitud acotada (RN-03).
	title := strings.TrimSpace(req.Title)
	switch {
	case title == "":
		addField("title", msgTitleRequired)
	case utf8.RuneCountInString(title) > maxTitleLength:
		addField("title", msgTitleTooLong)
	}

	// Descripción opcional: vacía o solo de espacios equivale a ausente (RN-04).
	var description *string
	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		switch {
		case utf8.RuneCountInString(trimmed) > maxDescriptionLength:
			addField("description", msgDescriptionTooLong)
		case trimmed != "":
			description = &trimmed
		}
	}

	// Prioridad obligatoria entre los valores en inglés del dominio (RN-05).
	if !isValidPriority(req.Priority) {
		addField("priority", msgInvalidPriority)
	}

	// Story Points opcionales: nil es "sin estimar" y el texto literal debe ser un entero de la
	// escala (RN-06, RN-07).
	var storyPoints *int
	if req.StoryPoints != nil {
		points, ok := parseStoryPoints(*req.StoryPoints)
		if ok {
			storyPoints = &points
		} else {
			addField("storyPoints", msgInvalidStoryPoints)
		}
	}

	// Criterios recortados en el orden recibido; cada uno informa su índice si es inválido (RN-08).
	criteria := make([]string, 0, len(req.AcceptanceCriteria))
	for i, raw := range req.AcceptanceCriteria {
		criterion := strings.TrimSpace(raw)
		field := fmt.Sprintf("acceptanceCriteria[%d]", i)
		switch {
		case criterion == "":
			addField(field, msgCriterionRequired)
		case utf8.RuneCountInString(criterion) > maxCriterionLength:
			addField(field, msgCriterionTooLong)
		default:
			criteria = append(criteria, criterion)
		}
	}

	if len(fields) > 0 {
		return ValidatedStory{}, fields
	}
	return ValidatedStory{
		Title:              title,
		Description:        description,
		Priority:           req.Priority,
		StoryPoints:        storyPoints,
		AcceptanceCriteria: criteria,
	}, nil
}

// ParseStoryFilter valida los valores repetibles de los filtros status y priority del listado, tal
// como los entrega c.QueryArray, y devuelve el StoryFilter o un FieldError por cada filtro con un
// valor no admitido, indicando los valores válidos (FR-034, FR-037).
func ParseStoryFilter(statuses, priorities []string) (domain.StoryFilter, []authdelivery.FieldError) {
	return domain.StoryFilter{}, nil
}

// isValidPriority indica si la prioridad es una de las admitidas, sin distinguir otras grafías.
func isValidPriority(priority domain.StoryPriority) bool {
	switch priority {
	case domain.StoryPriorityHigh, domain.StoryPriorityMedium, domain.StoryPriorityLow:
		return true
	}
	return false
}

// parseStoryPoints valida los Story Points sobre su texto literal: solo admite dígitos, de modo que
// los negativos, los decimales y la notación exponencial se rechazan antes de convertir, y el
// entero resultante debe pertenecer a la escala (RN-06).
func parseStoryPoints(value json.Number) (int, bool) {
	literal := string(value)
	if literal == "" || strings.IndexFunc(literal, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0, false
	}
	points, err := strconv.Atoi(literal)
	if err != nil || !domain.IsValidStoryPoints(points) {
		return 0, false
	}
	return points, true
}

// storyPointsScaleText enumera la escala de Story Points en español, separando con comas y con "y"
// antes del último valor, por ejemplo "0, 1, 2 y 3".
func storyPointsScaleText() string {
	values := make([]string, 0, len(domain.StoryPointsScale))
	for _, v := range domain.StoryPointsScale {
		values = append(values, strconv.Itoa(v))
	}
	if len(values) < 2 {
		return strings.Join(values, "")
	}
	last := len(values) - 1
	return strings.Join(values[:last], ", ") + " y " + values[last]
}
