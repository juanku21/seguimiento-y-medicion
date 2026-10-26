package delivery

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
)

// Límites de forma del alta: la longitud máxima de la actividad, en runas contadas tras recortar los
// espacios extremos, la cantidad máxima de decimales de las horas y el layout de la fecha de
// calendario (research.md secciones 5 y 6).
const (
	maxActivityLength = 300
	maxHoursDecimals  = 2
	entryDateLayout   = "2006-01-02"
)

// Mensajes de validación en español dirigidos a la persona, declarados en un único lugar y
// compartidos con el formulario del frontend (research.md sección 6). Una fecha ausente, mal formada
// o inexistente, como 2026-02-30, recibe el mismo mensaje.
const (
	msgHoursRange       = "Las horas deben ser mayores que 0 y como máximo 24."
	msgHoursMultiple    = "Las horas se cargan en múltiplos de 0,25 (un cuarto de hora)."
	msgHoursNumber      = "Las horas son obligatorias y deben ser un número."
	msgActivityRequired = "La actividad es obligatoria."
	msgActivityTooLong  = "La actividad admite como máximo 300 caracteres."
	msgEntryDateInvalid = "La fecha es obligatoria y debe tener el formato AAAA-MM-DD."
	msgPatchEmpty       = "Hay que indicar al menos un campo a modificar."
	msgPatchImmutable   = "La historia, el integrante y el sprint de un registro no se modifican; para mover horas, eliminá el registro y creá uno nuevo."
)

// Errores de ParseHours, parseEntryDate y normalizeActivity, cuyo texto es el mensaje que se
// informa en el campo correspondiente.
var (
	errHoursRange       = errors.New(msgHoursRange)
	errHoursMultiple    = errors.New(msgHoursMultiple)
	errHoursNumber      = errors.New(msgHoursNumber)
	errEntryDateInvalid = errors.New(msgEntryDateInvalid)
	errActivityRequired = errors.New(msgActivityRequired)
	errActivityTooLong  = errors.New(msgActivityTooLong)
)

// ParseHours valida las horas sobre su texto literal y las devuelve en centésimas de hora (RN-03),
// sin pasar por float64. Solo admite un signo menos opcional, dígitos y un punto decimal seguido de
// hasta dos dígitos: la ausencia, null, un booleano, un texto entre comillas, el exponente y los
// decimales de más se rechazan como no numéricos. Después exige el rango (0, 24] y, por último, el
// múltiplo de 0,25. Una parte entera que desborda el entero queda, por definición, fuera de rango.
func ParseHours(raw json.RawMessage) (int, error) {
	text := string(bytes.TrimSpace(raw))
	negative := strings.HasPrefix(text, "-")
	text = strings.TrimPrefix(text, "-")
	whole, fraction, hasPoint := strings.Cut(text, ".")
	validWhole := whole != "" && isDigits(whole)
	validFraction := !hasPoint || (fraction != "" && len(fraction) <= maxHoursDecimals && isDigits(fraction))
	if !validWhole || !validFraction {
		return 0, errHoursNumber
	}

	// Conversión a centésimas a partir de los dígitos: la fracción se completa con ceros a la derecha.
	units, err := strconv.Atoi(whole)
	if err != nil || units > domain.MaxEntryHundredths/100 {
		return 0, errHoursRange
	}
	fraction += strings.Repeat("0", maxHoursDecimals-len(fraction))
	cents, err := strconv.Atoi(fraction)
	if err != nil {
		return 0, errHoursNumber
	}
	hundredths := units*100 + cents

	// Reglas de magnitud: mayor que 0, como máximo 24 y múltiplo de un cuarto de hora (RN-03).
	switch {
	case negative || hundredths <= 0 || hundredths > domain.MaxEntryHundredths:
		return 0, errHoursRange
	case hundredths%domain.HoursStepHundredths != 0:
		return 0, errHoursMultiple
	}
	return hundredths, nil
}

// ValidateEntryInput valida el cuerpo del alta y devuelve los datos normalizados junto con un
// FieldError por cada campo inválido, todos a la vez (Principio VII). La actividad se devuelve
// recortada, la fecha a medianoche UTC y las horas en centésimas. Si hay algún campo inválido, se
// devuelve un EntryInput vacío.
func ValidateEntryInput(req EffortEntryInput) (domain.EntryInput, []authdelivery.FieldError) {
	fields := []authdelivery.FieldError{}
	addField := func(field, message string) {
		fields = append(fields, authdelivery.FieldError{Field: field, Message: message})
	}

	// Fecha de calendario y actividad, con las reglas compartidas con la modificación (RN-02).
	entryDate, err := parseEntryDate(req.EntryDate)
	if err != nil {
		addField("entryDate", err.Error())
	}
	activity, err := normalizeActivity(req.Activity)
	if err != nil {
		addField("activity", err.Error())
	}

	// Horas sobre su texto literal, con el mensaje que indique ParseHours (RN-03).
	hours, err := ParseHours(req.Hours)
	if err != nil {
		addField("hours", err.Error())
	}

	if len(fields) > 0 {
		return domain.EntryInput{}, fields
	}
	return domain.EntryInput{
		EntryDate:       entryDate,
		Activity:        activity,
		HoursHundredths: hours,
	}, nil
}

// ValidateEntryPatch valida el cuerpo de la modificación parcial de un registro y devuelve los
// cambios normalizados, o un FieldError por cada campo inválido, todos a la vez (RN-12, FR-023,
// Principio VII). Un campo ausente no genera cambio; los presentes, incluso en null, se validan con
// las mismas reglas que el alta. StoryID, UserID y SprintID se rechazan siempre que lleguen, y un
// cuerpo sin ningún campo editable se rechaza con field body. Si hay algún campo inválido no se
// devuelve ningún cambio.
func ValidateEntryPatch(req EffortEntryPatch) (domain.EntryPatch, []authdelivery.FieldError) {
	fields := []authdelivery.FieldError{}
	addField := func(field, message string) {
		fields = append(fields, authdelivery.FieldError{Field: field, Message: message})
	}
	var patch domain.EntryPatch

	// Fecha presente: texto con el formato AAAA-MM-DD y existente; null u otro tipo JSON se rechaza.
	if req.EntryDate != nil {
		entryDate, err := parseEntryDate(decodePatchString(req.EntryDate))
		if err != nil {
			addField("entryDate", err.Error())
		} else {
			patch.EntryDate = &entryDate
		}
	}

	// Actividad presente: recortada y de 1 a 300 runas; null u otro tipo JSON cuenta como vacía (RN-02).
	if req.Activity != nil {
		activity, err := normalizeActivity(decodePatchString(req.Activity))
		if err != nil {
			addField("activity", err.Error())
		} else {
			patch.Activity = &activity
		}
	}

	// Horas presentes: mismas reglas que el alta sobre su texto literal; null no es un número (RN-03).
	if req.Hours != nil {
		hours, err := ParseHours(req.Hours)
		if err != nil {
			addField("hours", err.Error())
		} else {
			patch.HoursHundredths = &hours
		}
	}

	// Historia, integrante y sprint son inmutables: se rechazan aunque lleguen en null (RN-12, FR-019).
	immutable := []struct {
		field string
		raw   json.RawMessage
	}{
		{"storyId", req.StoryID},
		{"userId", req.UserID},
		{"sprintId", req.SprintID},
	}
	for _, f := range immutable {
		if f.raw != nil {
			addField(f.field, msgPatchImmutable)
		}
	}

	if len(fields) > 0 {
		return domain.EntryPatch{}, fields
	}

	// Un cuerpo sin ningún campo editable no debe modificar el registro (FR-023).
	if req.EntryDate == nil && req.Activity == nil && req.Hours == nil {
		return domain.EntryPatch{}, []authdelivery.FieldError{{Field: "body", Message: msgPatchEmpty}}
	}
	return patch, nil
}

// ParseEntryFilter valida los parámetros de query de la consulta de esfuerzo del proyecto y
// devuelve el EntryFilter o un FieldError por cada filtro inválido (FR-038, FR-043). Firma mínima
// de T055; la implementación corresponde a T058.
func ParseEntryFilter(query url.Values) (domain.EntryFilter, []authdelivery.FieldError) {
	return domain.EntryFilter{}, []authdelivery.FieldError{{Field: "query", Message: "not implemented"}}
}

// parseEntryDate valida una fecha de calendario con el formato AAAA-MM-DD y existente; time.Parse
// sin zona la devuelve a medianoche UTC. El texto no se recorta, de modo que una fecha con espacios
// se rechaza.
func parseEntryDate(text string) (time.Time, error) {
	entryDate, err := time.Parse(entryDateLayout, text)
	if err != nil {
		return time.Time{}, errEntryDateInvalid
	}
	return entryDate, nil
}

// normalizeActivity recorta la actividad y exige que tenga de 1 a 300 runas (RN-02).
func normalizeActivity(text string) (string, error) {
	activity := strings.TrimSpace(text)
	switch {
	case activity == "":
		return "", errActivityRequired
	case utf8.RuneCountInString(activity) > maxActivityLength:
		return "", errActivityTooLong
	}
	return activity, nil
}

// decodePatchString decodifica un campo presente de la modificación como texto. El null y cualquier
// otro tipo JSON se entregan como texto vacío, para que cada campo aplique su propia regla de vacío
// o de formato.
func decodePatchString(raw json.RawMessage) string {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

// isDigits informa si el texto está compuesto únicamente por dígitos ASCII.
func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
