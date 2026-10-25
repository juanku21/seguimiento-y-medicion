package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pruebas unitarias obligatorias de CheckDailyCap (T012, RN-04) con la regla de data-model.md
// sección 6.4: la suma de lo ya registrado en la fecha y lo solicitado no supera 2400 centésimas
// (24 h); si lo supera, se rechaza con DailyCapExceededError y las centésimas todavía disponibles,
// que nunca son negativas.

// Completar exactamente el tope desde cero se acepta: 24 h es un extremo válido (006/US1, RN-04).
func TestCheckDailyCap_FullDayFromZero(t *testing.T) {
	err := CheckDailyCap(0, 2400)

	assert.NoError(t, err)
}

// Con 20 h registradas, sumar 4 h llega justo al tope y se acepta (006/US1, RN-04).
func TestCheckDailyCap_ReachesCapExactly(t *testing.T) {
	err := CheckDailyCap(2000, 400)

	assert.NoError(t, err)
}

// Con 20 h registradas, sumar 5 h supera el tope y se rechaza informando 4 h disponibles
// (006/US1, RN-04).
func TestCheckDailyCap_ExceedsCap(t *testing.T) {
	err := CheckDailyCap(2000, 500)

	var capErr *DailyCapExceededError
	require.ErrorAs(t, err, &capErr)
	assert.Equal(t, 400, capErr.AvailableHundredths)
}

// Con el tope ya completo, el mínimo de 0,25 h se rechaza informando 0 h disponibles
// (006/US1, RN-04).
func TestCheckDailyCap_CapAlreadyReached(t *testing.T) {
	err := CheckDailyCap(2400, 25)

	var capErr *DailyCapExceededError
	require.ErrorAs(t, err, &capErr)
	assert.Equal(t, 0, capErr.AvailableHundredths)
}

// Si lo ya registrado supera el tope, el disponible informado es 0 y nunca negativo; las variantes
// cubren distintos excesos previos (006/US1, RN-04).
func TestCheckDailyCap_AvailableNeverNegative(t *testing.T) {
	tests := []struct {
		name          string
		alreadyLogged int
		requested     int
	}{
		{name: "apenas por encima del tope", alreadyLogged: 2425, requested: 25},
		{name: "muy por encima del tope", alreadyLogged: 4800, requested: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckDailyCap(tt.alreadyLogged, tt.requested)

			var capErr *DailyCapExceededError
			require.ErrorAs(t, err, &capErr)
			assert.Equal(t, 0, capErr.AvailableHundredths)
		})
	}
}
