package delivery

import (
	"encoding/json"
	"errors"
	"time"
)

// NormalizeProjectName devuelve el nombre del proyecto listo para comparar sin distinguir
// mayúsculas ni espacios extremos (RN-05). Firma mínima de T017: la implementa T022.
func NormalizeProjectName(name string) string {
	return ""
}

// ValidateProjectName exige un nombre de entre 1 y 100 runas tras recortar los espacios extremos
// (FR-002, RN-04). Firma mínima de T017: la implementa T022.
func ValidateProjectName(name string) error {
	return errors.New("not implemented")
}

// NormalizeDescription recorta la descripción opcional y la convierte en nula si queda vacía
// (FR-003). Firma mínima de T017: la implementa T022.
func NormalizeDescription(description *string) *string {
	return nil
}

// ValidateDescription admite una descripción ausente o de hasta 1000 runas tras recortar los
// espacios extremos (FR-003). Firma mínima de T017: la implementa T022.
func ValidateDescription(description *string) error {
	return errors.New("not implemented")
}

// ValidateDateOrder rechaza una fecha de finalización prevista anterior a la de inicio; iguales es
// válido (FR-005, RN-06). Firma mínima de T017: la implementa T022.
func ValidateDateOrder(startDate, plannedEndDate time.Time) error {
	return errors.New("not implemented")
}

// ParseHoursPerStoryPoint valida el factor sobre su texto literal y lo devuelve en centésimas de
// hora, entre 1 y 4000, sin redondear (FR-006, RN-07). Firma mínima de T017: la implementa T022.
func ParseHoursPerStoryPoint(value json.Number) (int, error) {
	return 0, errors.New("not implemented")
}
