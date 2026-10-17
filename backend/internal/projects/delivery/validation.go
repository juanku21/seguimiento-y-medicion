package delivery

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Valores límite de la entrada de proyectos, declarados en un único lugar. El factor se expresa en
// centésimas de hora: 1 equivale a 0,01 y 4000 a 40 (RN-07). maxHoursPerStoryPointDecimals es la
// precisión admitida sobre el texto literal.
const (
	minProjectNameLength            = 1
	maxProjectNameLength            = 100
	maxDescriptionLength            = 1000
	minHoursPerStoryPointHundredths = 1
	maxHoursPerStoryPointHundredths = 4000
	maxHoursPerStoryPointDecimals   = 2
)

// Errores de validación con mensajes en español dirigidos a la persona. Los límites se toman de las
// constantes para no repetirlos en el texto; el factor comparte un único mensaje que indica el rango
// y la precisión admitidos para cualquier valor rechazado, incluido el ausente.
var (
	errInvalidProjectName = fmt.Errorf("El nombre debe tener entre %d y %d caracteres.",
		minProjectNameLength, maxProjectNameLength)
	errInvalidDescription = fmt.Errorf("La descripción admite hasta %d caracteres.",
		maxDescriptionLength)
	errPlannedEndBeforeStart = errors.New(
		"La fecha de finalización prevista no puede ser anterior a la fecha de inicio.")
	errInvalidHoursPerStoryPoint = fmt.Errorf(
		"El factor de horas por Story Point debe ser mayor que 0 y menor o igual a %d, con hasta %d decimales.",
		maxHoursPerStoryPointHundredths/100, maxHoursPerStoryPointDecimals)
)

// NormalizeProjectName devuelve el nombre del proyecto listo para comparar sin distinguir
// mayúsculas ni espacios extremos (RN-05).
func NormalizeProjectName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// ValidateProjectName exige un nombre de entre minProjectNameLength y maxProjectNameLength runas
// tras recortar los espacios extremos; vacío o solo de espacios equivale a ausente (FR-002, RN-04).
func ValidateProjectName(name string) error {
	length := utf8.RuneCountInString(strings.TrimSpace(name))
	if length < minProjectNameLength || length > maxProjectNameLength {
		return errInvalidProjectName
	}
	return nil
}

// NormalizeDescription recorta la descripción opcional y la convierte en nula si queda vacía
// (FR-003).
func NormalizeDescription(description *string) *string {
	if description == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*description)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// ValidateDescription admite una descripción ausente o de hasta maxDescriptionLength runas tras
// recortar los espacios extremos (FR-003).
func ValidateDescription(description *string) error {
	if description == nil {
		return nil
	}
	if utf8.RuneCountInString(strings.TrimSpace(*description)) > maxDescriptionLength {
		return errInvalidDescription
	}
	return nil
}

// ValidateDateOrder rechaza una fecha de finalización prevista anterior a la de inicio; iguales es
// válido (FR-005, RN-06).
func ValidateDateOrder(startDate, plannedEndDate time.Time) error {
	if plannedEndDate.Before(startDate) {
		return errPlannedEndBeforeStart
	}
	return nil
}

// ParseHoursPerStoryPoint valida el factor sobre su texto literal y lo devuelve en centésimas de
// hora, entre minHoursPerStoryPointHundredths y maxHoursPerStoryPointHundredths, sin redondear (FR-006,
// RN-07). Solo admite dígitos con un punto decimal opcional seguido de hasta dos dígitos, de modo
// que los negativos, la notación exponencial y los decimales de más se rechazan antes de convertir.
// La parte entera se acota antes de multiplicar para que un texto enorme no desborde el entero.
func ParseHoursPerStoryPoint(value json.Number) (int, error) {
	whole, fraction, hasPoint := strings.Cut(string(value), ".")
	validWhole := whole != "" && isDigits(whole)
	validFraction := !hasPoint || (fraction != "" &&
		len(fraction) <= maxHoursPerStoryPointDecimals && isDigits(fraction))
	if !validWhole || !validFraction {
		return 0, errInvalidHoursPerStoryPoint
	}

	units, err := strconv.Atoi(whole)
	if err != nil || units > maxHoursPerStoryPointHundredths/100 {
		return 0, errInvalidHoursPerStoryPoint
	}
	fraction += strings.Repeat("0", maxHoursPerStoryPointDecimals-len(fraction))
	cents, err := strconv.Atoi(fraction)
	if err != nil {
		return 0, errInvalidHoursPerStoryPoint
	}

	hundredths := units*100 + cents
	if hundredths < minHoursPerStoryPointHundredths || hundredths > maxHoursPerStoryPointHundredths {
		return 0, errInvalidHoursPerStoryPoint
	}
	return hundredths, nil
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
