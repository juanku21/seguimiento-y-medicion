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
	msgInvalidPriority     = "Los valores admitidos son Alta, Media y Baja."
	msgInvalidStoryPoints  = "Los valores admitidos son " + storyPointsScaleText() + "."
	msgStoryPointsRequired = "Los Story Points son obligatorios; para dejar la historia sin estimar se envía null."
	msgCriterionRequired   = "Cada criterio de aceptación debe tener contenido."
	msgCriterionTooLong    = fmt.Sprintf("Cada criterio de aceptación admite como máximo %d caracteres.",
		maxCriterionLength)
	msgTitleNotText           = "El título debe ser un texto."
	msgDescriptionNotText     = "La descripción debe ser un texto o null."
	msgCriteriaNotList        = "Los criterios de aceptación deben ser una lista de textos."
	msgCriteriaNull           = "Los criterios de aceptación no admiten null; para quitarlos todos se envía una lista vacía."
	msgStatusNotEditable      = "El estado de una historia no se cambia desde el backlog; lo cambia la gestión de sprints."
	msgStoryPointsNotEditable = "Los Story Points no se cambian por esta vía; se usa la estimación de la historia."
	msgPatchWithoutFields     = "La solicitud no incluye ningún campo a modificar."
	msgInvalidStatusFilter    = "Los valores admitidos para el estado son pending, in_progress y completed."
	msgInvalidPriorityFilter  = "Los valores admitidos para la prioridad son high, medium y low."
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
	title, msg := validateTitle(req.Title)
	if msg != "" {
		addField("title", msg)
	}

	// Descripción opcional: vacía o solo de espacios equivale a ausente (RN-04).
	var description *string
	if req.Description != nil {
		description, msg = validateDescription(*req.Description)
		if msg != "" {
			addField("description", msg)
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
	criteria, criteriaFields := validateCriteria(req.AcceptanceCriteria)
	fields = append(fields, criteriaFields...)

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

// ValidateStoryPointsInput valida el cuerpo de la estimación y devuelve los Story Points, nil para
// "sin estimar", o un FieldError si el campo falta o su valor está fuera de la escala (FR-016,
// FR-018, FR-019). El campo ausente llega como mensaje vacío y el null como su literal; cualquier
// otro valor se valida sobre su texto con la misma regla de escala que la creación (research.md
// sección 8).
func ValidateStoryPointsInput(req StoryPointsInput) (*int, []authdelivery.FieldError) {
	switch string(req.StoryPoints) {
	case "":
		return nil, []authdelivery.FieldError{{Field: "storyPoints", Message: msgStoryPointsRequired}}
	case "null":
		return nil, nil
	}
	points, ok := parseStoryPoints(json.Number(req.StoryPoints))
	if !ok {
		return nil, []authdelivery.FieldError{{Field: "storyPoints", Message: msgInvalidStoryPoints}}
	}
	return &points, nil
}

// ValidateStoryPatch decodifica y valida el cuerpo del PATCH y devuelve los cambios a aplicar, o un
// FieldError por cada campo inválido, todos a la vez (FR-020, FR-021, FR-023). Un campo ausente no
// genera cambio y los presentes se validan con las mismas reglas que la creación. Status y
// StoryPoints se rechazan siempre, y un cuerpo sin campos editables se rechaza con field body
// (research.md sección 6). Si hay algún campo inválido no se devuelve ningún cambio.
func ValidateStoryPatch(req StoryPatch) (domain.StoryChanges, []authdelivery.FieldError) {
	fields := []authdelivery.FieldError{}
	addField := func(field, message string) {
		fields = append(fields, authdelivery.FieldError{Field: field, Message: message})
	}
	var changes domain.StoryChanges

	// Título presente: debe ser texto y cumplir RN-03; null cuenta como vacío y se rechaza.
	if req.Title != nil {
		raw, ok := decodePatchString(req.Title)
		title, msg := validateTitle(raw)
		switch {
		case !ok:
			addField("title", msgTitleNotText)
		case msg != "":
			addField("title", msg)
		default:
			changes.Title = &title
		}
	}

	// Descripción presente: null, vacía o solo espacios la borran; si no, cumple RN-04.
	if req.Description != nil {
		raw, ok := decodePatchString(req.Description)
		description, msg := validateDescription(raw)
		switch {
		case !ok:
			addField("description", msgDescriptionNotText)
		case msg != "":
			addField("description", msg)
		default:
			changes.DescriptionSet = true
			changes.Description = description
		}
	}

	// Prioridad presente: debe ser uno de los valores del dominio; null o vacía se rechaza (RN-05).
	if req.Priority != nil {
		raw, ok := decodePatchString(req.Priority)
		priority := domain.StoryPriority(raw)
		if ok && isValidPriority(priority) {
			changes.Priority = &priority
		} else {
			addField("priority", msgInvalidPriority)
		}
	}

	// Criterios presentes: null se rechaza, [] vacía la lista y cada criterio cumple RN-08.
	if req.AcceptanceCriteria != nil {
		var raw []string
		switch {
		case string(req.AcceptanceCriteria) == "null":
			addField("acceptanceCriteria", msgCriteriaNull)
		case json.Unmarshal(req.AcceptanceCriteria, &raw) != nil:
			addField("acceptanceCriteria", msgCriteriaNotList)
		default:
			criteria, criteriaFields := validateCriteria(raw)
			fields = append(fields, criteriaFields...)
			changes.AcceptanceCriteria = &criteria
		}
	}

	// Estado y Story Points no se editan por esta vía, aunque lleguen en null (FR-027, RN-10).
	if req.Status != nil {
		addField("status", msgStatusNotEditable)
	}
	if req.StoryPoints != nil {
		addField("storyPoints", msgStoryPointsNotEditable)
	}

	if len(fields) > 0 {
		return domain.StoryChanges{}, fields
	}

	// Un cuerpo sin ningún campo editable no debe actualizar la historia.
	noEditableFields := req.Title == nil && req.Description == nil &&
		req.Priority == nil && req.AcceptanceCriteria == nil
	if noEditableFields {
		return domain.StoryChanges{}, []authdelivery.FieldError{{Field: "body", Message: msgPatchWithoutFields}}
	}
	return changes, nil
}

// decodePatchString decodifica un campo presente del PATCH como texto. El null se entrega como
// texto vacío, para que cada campo aplique su propia regla de vacío; cualquier otro tipo JSON
// devuelve false.
func decodePatchString(raw json.RawMessage) (string, bool) {
	if string(raw) == "null" {
		return "", true
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

// validateTitle recorta el título y devuelve el mensaje de error si queda vacío o supera el máximo,
// o un mensaje vacío si es válido (RN-03). La comparten la creación y la modificación.
func validateTitle(raw string) (string, string) {
	title := strings.TrimSpace(raw)
	switch {
	case title == "":
		return "", msgTitleRequired
	case utf8.RuneCountInString(title) > maxTitleLength:
		return "", msgTitleTooLong
	}
	return title, ""
}

// validateDescription recorta la descripción y devuelve nil si queda vacía, o el mensaje de error
// si supera el máximo (RN-04). La comparten la creación y la modificación.
func validateDescription(raw string) (*string, string) {
	description := strings.TrimSpace(raw)
	switch {
	case utf8.RuneCountInString(description) > maxDescriptionLength:
		return nil, msgDescriptionTooLong
	case description == "":
		return nil, ""
	}
	return &description, ""
}

// validateCriteria recorta los criterios en el orden recibido y devuelve un FieldError con el
// índice de cada criterio vacío o demasiado largo (RN-08). La lista devuelta nunca es nil, de modo
// que una lista vacía se distingue de una ausente. La comparten la creación y la modificación.
func validateCriteria(raw []string) ([]string, []authdelivery.FieldError) {
	criteria := make([]string, 0, len(raw))
	fields := []authdelivery.FieldError{}
	for i, value := range raw {
		criterion := strings.TrimSpace(value)
		field := fmt.Sprintf("acceptanceCriteria[%d]", i)
		switch {
		case criterion == "":
			fields = append(fields, authdelivery.FieldError{Field: field, Message: msgCriterionRequired})
		case utf8.RuneCountInString(criterion) > maxCriterionLength:
			fields = append(fields, authdelivery.FieldError{Field: field, Message: msgCriterionTooLong})
		default:
			criteria = append(criteria, criterion)
		}
	}
	return criteria, fields
}

// ParseStoryFilter valida los valores repetibles de los filtros status y priority del listado, tal
// como los entrega c.QueryArray, y devuelve el StoryFilter o un FieldError por cada filtro con un
// valor no admitido, indicando los valores válidos (FR-034, FR-037).
func ParseStoryFilter(statuses, priorities []string) (domain.StoryFilter, []authdelivery.FieldError) {
	fields := []authdelivery.FieldError{}

	// Cada filtro se convierte por separado para informar los dos a la vez si ambos son inválidos.
	statusFilter, statusOK := parseFilterValues(statuses, isValidStatus)
	if !statusOK {
		fields = append(fields, authdelivery.FieldError{Field: "status", Message: msgInvalidStatusFilter})
	}
	priorityFilter, priorityOK := parseFilterValues(priorities, isValidPriority)
	if !priorityOK {
		fields = append(fields, authdelivery.FieldError{Field: "priority", Message: msgInvalidPriorityFilter})
	}

	if len(fields) > 0 {
		return domain.StoryFilter{}, fields
	}
	return domain.StoryFilter{Statuses: statusFilter, Priorities: priorityFilter}, nil
}

// parseFilterValues convierte los valores de un filtro repetible al tipo del dominio y devuelve
// false si alguno no es admitido; la comparación es exacta, sin aceptar otras grafías. Los valores
// repetidos se descartan conservando el orden de su primera aparición, de modo que la cláusula IN
// reciba cada valor admitido una sola vez (hallazgo de seguridad: filtros repetibles sin tope).
func parseFilterValues[T ~string](values []string, isValid func(T) bool) ([]T, bool) {
	parsed := []T{}
	seen := map[T]bool{}
	for _, v := range values {
		value := T(v)
		if !isValid(value) {
			return nil, false
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		parsed = append(parsed, value)
	}
	return parsed, true
}

// isValidStatus indica si el estado es uno de los admitidos, sin distinguir otras grafías.
func isValidStatus(status domain.StoryStatus) bool {
	switch status {
	case domain.StoryStatusPending, domain.StoryStatusInProgress, domain.StoryStatusCompleted:
		return true
	}
	return false
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
