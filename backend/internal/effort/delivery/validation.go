package delivery

import (
	"bytes"
	"encoding/json"
	"errors"
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
)

// Errores de ParseHours, cuyo texto es el mensaje que se informa en el campo hours.
var (
	errHoursRange    = errors.New(msgHoursRange)
	errHoursMultiple = errors.New(msgHoursMultiple)
	errHoursNumber   = errors.New(msgHoursNumber)
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

	// Fecha de calendario obligatoria con el formato AAAA-MM-DD y existente; time.Parse sin zona la
	// devuelve a medianoche UTC. El texto no se recorta, de modo que una fecha con espacios se rechaza.
	entryDate, err := time.Parse(entryDateLayout, req.EntryDate)
	if err != nil {
		addField("entryDate", msgEntryDateInvalid)
	}

	// Actividad obligatoria, recortada y de 1 a 300 runas (RN-02).
	activity := strings.TrimSpace(req.Activity)
	switch {
	case activity == "":
		addField("activity", msgActivityRequired)
	case utf8.RuneCountInString(activity) > maxActivityLength:
		addField("activity", msgActivityTooLong)
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
// cambios normalizados, o un FieldError por cada campo inválido (RN-12, FR-023). Firma mínima de
// T042: la implementa T049.
func ValidateEntryPatch(req EffortEntryPatch) (domain.EntryPatch, []authdelivery.FieldError) {
	return domain.EntryPatch{}, nil
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
