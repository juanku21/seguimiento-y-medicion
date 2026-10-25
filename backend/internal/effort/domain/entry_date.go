package domain

import (
	"errors"
	"time"
)

// ValidateEntryDate verifica que la fecha de un registro no sea posterior a hoy ni anterior al inicio
// del proyecto; ambos extremos son válidos (data-model.md sección 6.3, RN-05). Firma mínima de T011:
// la implementación corresponde a T021.
func ValidateEntryDate(date, projectStart, today time.Time) error {
	return errors.New("not implemented")
}
