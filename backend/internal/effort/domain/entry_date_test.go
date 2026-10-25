package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pruebas unitarias obligatorias de ValidateEntryDate (T011, RN-05, SC-007) con la regla de
// data-model.md sección 6.3: una fecha posterior a hoy se rechaza con ErrEntryDateInFuture, una
// anterior al inicio del proyecto con EntryDateBeforeProjectStartError y la fecha mínima, y ambos
// extremos (hoy y el día de inicio) se aceptan. Las fechas son de calendario, a medianoche UTC
// (RC-08), y "hoy" se inyecta para que las pruebas no dependan del reloj real.

// entryDay construye una fecha de calendario a medianoche UTC, como se maneja entry_date en Go.
func entryDay(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// entryDateToday y entryDateProjectStart son el "hoy" y el inicio del proyecto fijos de las pruebas.
var (
	entryDateToday        = entryDay(2026, time.October, 25)
	entryDateProjectStart = entryDay(2026, time.September, 1)
)

// La fecha de hoy es un extremo válido y se acepta (006/US1, RN-05, SC-007).
func TestValidateEntryDate_Today(t *testing.T) {
	err := ValidateEntryDate(entryDateToday, entryDateProjectStart, entryDateToday)

	assert.NoError(t, err)
}

// Una fecha posterior a hoy se rechaza como futura: mañana es la primera fecha inválida
// (006/US1, RN-05, SC-007).
func TestValidateEntryDate_Tomorrow(t *testing.T) {
	tomorrow := entryDateToday.AddDate(0, 0, 1)

	err := ValidateEntryDate(tomorrow, entryDateProjectStart, entryDateToday)

	assert.ErrorIs(t, err, ErrEntryDateInFuture)
}

// El día de inicio del proyecto es un extremo válido y se acepta (006/US1, RN-05, SC-007).
func TestValidateEntryDate_ProjectStartDay(t *testing.T) {
	err := ValidateEntryDate(entryDateProjectStart, entryDateProjectStart, entryDateToday)

	assert.NoError(t, err)
}

// El día anterior al inicio del proyecto se rechaza e informa como fecha mínima el día de inicio
// (006/US1, RN-05, SC-007).
func TestValidateEntryDate_DayBeforeProjectStart(t *testing.T) {
	dayBefore := entryDateProjectStart.AddDate(0, 0, -1)

	err := ValidateEntryDate(dayBefore, entryDateProjectStart, entryDateToday)

	var startErr *EntryDateBeforeProjectStartError
	require.ErrorAs(t, err, &startErr)
	assert.True(t, startErr.MinDate.Equal(entryDateProjectStart),
		"MinDate = %v, se esperaba %v", startErr.MinDate, entryDateProjectStart)
}

// Con un proyecto cuyo inicio es posterior a hoy no existe fecha válida: lo anterior al inicio se
// rechaza por el inicio y lo posterior a hoy, por futuro (006/US1, RN-05, SC-007).
func TestValidateEntryDate_FutureProjectStart(t *testing.T) {
	futureStart := entryDateToday.AddDate(0, 0, 10)

	// Las fechas de hasta hoy son anteriores al inicio y se rechazan con la fecha mínima del proyecto.
	for _, tc := range []struct {
		name string
		date time.Time
	}{
		{name: "pasado", date: entryDateToday.AddDate(0, 0, -1)},
		{name: "hoy", date: entryDateToday},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEntryDate(tc.date, futureStart, entryDateToday)

			var startErr *EntryDateBeforeProjectStartError
			require.ErrorAs(t, err, &startErr)
			assert.True(t, startErr.MinDate.Equal(futureStart),
				"MinDate = %v, se esperaba %v", startErr.MinDate, futureStart)
		})
	}

	// Las fechas posteriores a hoy se rechazan como futuras, incluso el día de inicio y los siguientes.
	for _, tc := range []struct {
		name string
		date time.Time
	}{
		{name: "mañana", date: entryDateToday.AddDate(0, 0, 1)},
		{name: "día de inicio", date: futureStart},
		{name: "después del inicio", date: futureStart.AddDate(0, 0, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEntryDate(tc.date, futureStart, entryDateToday)

			assert.ErrorIs(t, err, ErrEntryDateInFuture)
		})
	}
}
