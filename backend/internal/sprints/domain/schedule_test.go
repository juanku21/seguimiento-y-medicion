package domain

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Pruebas unitarias obligatorias de Schedule (T071; RN-23, FR-053; data-model.md sección 6). No
// llevan la etiqueta de un escenario de la spec: los escenarios US6-4 a US6-6 se cubren con las
// pruebas de integración TestUS6_DiasRestantes, TestUS6_TerminaHoy y TestUS6_Vencido.

// scheduleZone es una zona fija equivalente a APP_TIMEZONE (America/Argentina/Buenos_Aires, UTC−3),
// declarada sin la base de zonas horarias para que la prueba no dependa del sistema operativo.
var scheduleZone = time.FixedZone("UTC-3", -3*60*60)

// intPtr devuelve un puntero al entero dado, para expresar los valores esperados no nulos.
func intPtr(v int) *int {
	return &v
}

// showDays representa un valor opcional de días como texto, "nil" si falta, para que un fallo
// muestre los números y no las direcciones de los punteros.
func showDays(p *int) string {
	if p == nil {
		return "nil"
	}
	return strconv.Itoa(*p)
}

// Cada caso fija la fecha de fin del sprint a medianoche UTC, como se persiste, y un instante "now";
// se verifica que solo uno de los dos valores venga informado y que ninguno sea negativo.
func TestSchedule(t *testing.T) {
	endDate := time.Date(2026, time.October, 14, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name          string
		now           time.Time
		wantRemaining *int
		wantOverdue   *int
	}{
		{
			name:          "faltan diez días",
			now:           time.Date(2026, time.October, 4, 12, 0, 0, 0, scheduleZone),
			wantRemaining: intPtr(10),
			wantOverdue:   nil,
		},
		{
			name:          "el día de fin vale cero",
			now:           time.Date(2026, time.October, 14, 12, 0, 0, 0, scheduleZone),
			wantRemaining: intPtr(0),
			wantOverdue:   nil,
		},
		{
			name:          "vencido hace tres días",
			now:           time.Date(2026, time.October, 17, 12, 0, 0, 0, scheduleZone),
			wantRemaining: nil,
			wantOverdue:   intPtr(3),
		},
		{
			// 2026-10-15 02:00 UTC es todavía 2026-10-14 23:00 en la zona del sistema: cuenta el día
			// de calendario local, no el de UTC.
			name:          "borde de medianoche: en UTC ya es el día siguiente pero en la zona es el día de fin",
			now:           time.Date(2026, time.October, 15, 2, 0, 0, 0, time.UTC),
			wantRemaining: intPtr(0),
			wantOverdue:   nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			remaining, overdue := Schedule(endDate, tc.now, scheduleZone)
			assert.Equal(t, showDays(tc.wantRemaining), showDays(remaining), "remainingDays con now=%s", tc.now.Format(time.RFC3339))
			assert.Equal(t, showDays(tc.wantOverdue), showDays(overdue), "overdueDays con now=%s", tc.now.Format(time.RFC3339))
		})
	}
}
