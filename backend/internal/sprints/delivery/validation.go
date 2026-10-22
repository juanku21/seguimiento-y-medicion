package delivery

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Longitudes máximas del nombre y del Sprint Goal, en runas contadas tras recortar los espacios
// extremos y declaradas en un único lugar (RN-03).
const (
	maxNameLength = 100
	maxGoalLength = 500
)

// Mensajes de validación en español dirigidos a la persona (data-model.md, reglas de validación).
// Los límites se toman de las constantes para no repetirlos en el texto. Una fecha inexistente,
// como 2026-02-30, recibe el mismo mensaje que una mal formada.
var (
	msgNameRequired         = "El nombre es obligatorio."
	msgNameTooLong          = fmt.Sprintf("El nombre admite como máximo %d caracteres.", maxNameLength)
	msgGoalRequired         = "El Sprint Goal es obligatorio."
	msgGoalTooLong          = fmt.Sprintf("El Sprint Goal admite como máximo %d caracteres.", maxGoalLength)
	msgStartDateRequired    = "La fecha de inicio es obligatoria."
	msgStartDateInvalid     = "La fecha de inicio debe ser una fecha válida con el formato AAAA-MM-DD."
	msgEndDateRequired      = "La fecha de fin es obligatoria."
	msgEndDateInvalid       = "La fecha de fin debe ser una fecha válida con el formato AAAA-MM-DD."
	msgEndDateNotAfterStart = "La fecha de fin debe ser posterior a la de inicio."
)

// ValidateSprintInput aplica las reglas RN-03 y RN-04 al cuerpo de la creación y la modificación de
// un sprint y devuelve los datos normalizados junto con un FieldError por cada campo inválido,
// todos a la vez (FR-011). El nombre y el Sprint Goal se devuelven recortados y las fechas a
// medianoche UTC. El orden de las fechas solo se verifica cuando ambas son válidas. Si hay algún
// campo inválido, se devuelve un SprintInput vacío.
func ValidateSprintInput(req SprintInput) (domain.SprintInput, []authdelivery.FieldError) {
	fields := []authdelivery.FieldError{}
	addField := func(field, message string) {
		fields = append(fields, authdelivery.FieldError{Field: field, Message: message})
	}

	// Nombre y Sprint Goal obligatorios, recortados y con longitud acotada (RN-03).
	name, msg := validateText(req.Name, maxNameLength, msgNameRequired, msgNameTooLong)
	if msg != "" {
		addField("name", msg)
	}
	goal, msg := validateText(req.Goal, maxGoalLength, msgGoalRequired, msgGoalTooLong)
	if msg != "" {
		addField("goal", msg)
	}

	// Fechas obligatorias con el formato YYYY-MM-DD; el fin debe ser estrictamente posterior al
	// inicio, comparación que solo tiene sentido si ambas se pudieron convertir (RN-04).
	startDate, startMsg := parseSprintDate(req.StartDate, msgStartDateRequired, msgStartDateInvalid)
	if startMsg != "" {
		addField("startDate", startMsg)
	}
	endDate, endMsg := parseSprintDate(req.EndDate, msgEndDateRequired, msgEndDateInvalid)
	switch {
	case endMsg != "":
		addField("endDate", endMsg)
	case startMsg == "" && !endDate.After(startDate):
		addField("endDate", msgEndDateNotAfterStart)
	}

	if len(fields) > 0 {
		return domain.SprintInput{}, fields
	}
	return domain.SprintInput{
		Name:      name,
		Goal:      goal,
		StartDate: startDate,
		EndDate:   endDate,
	}, nil
}

// validateText recorta un texto obligatorio y devuelve el mensaje de error si queda vacío o supera
// el máximo de runas, o un mensaje vacío si es válido (RN-03).
func validateText(raw string, maxLength int, msgRequired, msgTooLong string) (string, string) {
	text := strings.TrimSpace(raw)
	switch {
	case text == "":
		return "", msgRequired
	case utf8.RuneCountInString(text) > maxLength:
		return "", msgTooLong
	}
	return text, ""
}

// parseSprintDate convierte una fecha de calendario con el layout YYYY-MM-DD (research.md sección
// 4) a medianoche UTC. Devuelve msgRequired si la fecha falta y msgInvalid si tiene otro formato o
// no existe; el texto no se recorta, de modo que una fecha con espacios se rechaza.
func parseSprintDate(raw, msgRequired, msgInvalid string) (time.Time, string) {
	if raw == "" {
		return time.Time{}, msgRequired
	}
	date, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return time.Time{}, msgInvalid
	}
	return date, ""
}
