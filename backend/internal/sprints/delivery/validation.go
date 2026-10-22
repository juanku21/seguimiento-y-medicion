package delivery

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Longitudes máximas del nombre y del Sprint Goal, en runas contadas tras recortar los espacios
// extremos y declaradas en un único lugar (RN-03).
const (
	maxNameLength = 100
	maxGoalLength = 500
)

// maxStoryIDs es la cantidad máxima de historias de un lote de asignación o de baja; no es un
// requisito de la spec sino una cota de entrada (research.md sección 8, Principio VII).
const maxStoryIDs = 100

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
	msgStoryIDsRequired     = "Hay que indicar al menos una historia."
	msgStoryIDsTooMany      = fmt.Sprintf("Se admiten como máximo %d historias por solicitud.", maxStoryIDs)
	msgStoryIDInvalid       = "El identificador de la historia debe ser un UUID válido."
	msgStoryIDRepeated      = "La historia está repetida en la solicitud."
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

// Campos del FieldError de un lote de historias: el arreglo del cuerpo de la asignación, que se
// señala por posición, y el parámetro repetible de la baja.
const (
	storyIDsBodyField  = "storyIds"
	storyIDsQueryField = "storyId"
)

// ValidateStoryIDsInput valida la lista de identificadores del cuerpo de la asignación y devuelve los
// UUID en el orden recibido. La cantidad se informa en el campo storyIds y cada identificador
// inválido o repetido en storyIds[i]. Si hay algún problema, la lista devuelta es nil.
func ValidateStoryIDsInput(req StoryIDsInput) ([]uuid.UUID, []authdelivery.FieldError) {
	return validateStoryIDs(req.StoryIDs, storyIDsBodyField, func(i int) string {
		return fmt.Sprintf("%s[%d]", storyIDsBodyField, i)
	})
}

// ValidateStoryIDParams valida los parámetros storyId repetidos de la baja, tal como los entrega
// c.QueryArray, y devuelve los UUID en el orden recibido. Todos los problemas se informan en el
// campo storyId. Si hay algún problema, la lista devuelta es nil.
func ValidateStoryIDParams(values []string) ([]uuid.UUID, []authdelivery.FieldError) {
	return validateStoryIDs(values, storyIDsQueryField, func(int) string {
		return storyIDsQueryField
	})
}

// validateStoryIDs aplica las cotas de entrada de un lote de historias (research.md sección 8,
// Principio VII): entre 1 y maxStoryIDs identificadores, UUID válidos y sin repetidos. Una cantidad
// fuera de rango se informa sola en listField, sin revisar cada elemento; si no, cada identificador
// inválido o repetido recibe su FieldError en itemField(i). Los repetidos se comparan ya convertidos,
// de modo que dos textos del mismo UUID con distinta capitalización cuentan como repetidos.
func validateStoryIDs(values []string, listField string, itemField func(i int) string) ([]uuid.UUID, []authdelivery.FieldError) {
	switch {
	case len(values) == 0:
		return nil, []authdelivery.FieldError{{Field: listField, Message: msgStoryIDsRequired}}
	case len(values) > maxStoryIDs:
		return nil, []authdelivery.FieldError{{Field: listField, Message: msgStoryIDsTooMany}}
	}

	ids := make([]uuid.UUID, 0, len(values))
	seen := make(map[uuid.UUID]struct{}, len(values))
	fields := []authdelivery.FieldError{}
	for i, value := range values {
		id, err := uuid.Parse(value)
		if err != nil {
			fields = append(fields, authdelivery.FieldError{Field: itemField(i), Message: msgStoryIDInvalid})
			continue
		}
		if _, repeated := seen[id]; repeated {
			fields = append(fields, authdelivery.FieldError{Field: itemField(i), Message: msgStoryIDRepeated})
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	if len(fields) > 0 {
		return nil, fields
	}
	return ids, nil
}

// Mensajes de validación del estado destino del avance de una historia, señalados en el campo status.
var (
	msgStatusRequired = "El estado es obligatorio."
	msgStatusInvalid  = "Los valores admitidos para el estado son pending, in_progress y completed."
)

// ValidateStoryStatusInput valida el cuerpo del avance de una historia y devuelve el estado destino
// (contracts/openapi.yaml, StoryStatusInput). El estado es obligatorio y debe ser exactamente uno de
// los tres valores de StoryStatus, sin recortar espacios ni ignorar mayúsculas. Si la transición está
// admitida lo decide después ValidateStoryProgress, así que pending es un valor válido para el cuerpo.
// Ante cualquier problema devuelve un estado vacío y un único FieldError en status.
func ValidateStoryStatusInput(req StoryStatusInput) (backlogdomain.StoryStatus, []authdelivery.FieldError) {
	status := backlogdomain.StoryStatus(req.Status)
	switch status {
	case "":
		return "", []authdelivery.FieldError{{Field: "status", Message: msgStatusRequired}}
	case backlogdomain.StoryStatusPending, backlogdomain.StoryStatusInProgress, backlogdomain.StoryStatusCompleted:
		return status, nil
	default:
		return "", []authdelivery.FieldError{{Field: "status", Message: msgStatusInvalid}}
	}
}

// Campo y mensaje del rechazo del filtro de estado del listado de sprints, que enumera los valores
// admitidos.
const (
	sprintStatusQueryField       = "status"
	msgSprintStatusFilterInvalid = "Los valores admitidos para el estado son planned, active y closed."
)

// ParseSprintStatusFilter valida los valores repetibles del filtro status del listado de sprints, tal
// como los entrega c.QueryArray, y devuelve los estados del dominio o un FieldError en status que
// indica los valores admitidos (FR-050, FR-051). Sin valores devuelve una lista vacía y no nil,
// porque appliedFilters.status nunca es null. La comparación es exacta, como el enum del contrato:
// no se recortan espacios ni se ignoran mayúsculas. Ante un valor no admitido devuelve nil y un
// único FieldError.
func ParseSprintStatusFilter(values []string) ([]domain.SprintStatus, []authdelivery.FieldError) {
	statuses := make([]domain.SprintStatus, 0, len(values))
	for _, value := range values {
		status := domain.SprintStatus(value)
		switch status {
		case domain.SprintStatusPlanned, domain.SprintStatusActive, domain.SprintStatusClosed:
			statuses = append(statuses, status)
		default:
			return nil, []authdelivery.FieldError{{Field: sprintStatusQueryField, Message: msgSprintStatusFilterInvalid}}
		}
	}
	return statuses, nil
}
