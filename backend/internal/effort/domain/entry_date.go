package domain

import "time"

// ValidateEntryDate verifica que la fecha de un registro no sea posterior a hoy ni anterior al inicio
// del proyecto; ambos extremos son válidos (data-model.md sección 6.3, RN-05). Se compara primero
// contra hoy, de modo que una fecha futura y anterior al inicio se rechaza como futura; si es
// anterior al inicio, el error lleva como fecha mínima el día de inicio del proyecto.
func ValidateEntryDate(date, projectStart, today time.Time) error {
	if date.After(today) {
		return ErrEntryDateInFuture
	}
	if date.Before(projectStart) {
		return &EntryDateBeforeProjectStartError{MinDate: projectStart}
	}
	return nil
}
