package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
)

// Escenario: un cero calculado no es un dato ausente (008/US1, RN-17, RN-19)
func TestUS1_CeroDisponibleDistintoDeAusente(t *testing.T) {
	zero := domain.Available(0)

	assert.Equal(t, domain.ValueAvailable, zero.Status)
	assert.Equal(t, 0, zero.Value)
	assert.Nil(t, zero.Finality)

	// Los tres estados deben ser distintos entre sí.
	assert.NotEqual(t, domain.NotCalculable(), zero)
	assert.NotEqual(t, domain.NotApplicable(), zero)
	assert.NotEqual(t, domain.NotApplicable(), domain.NotCalculable())
}

// Escenario: constructores de estado sin valor (008/US1, RN-19)
func TestUS1_EstadosSinValor(t *testing.T) {
	assert.Equal(t, domain.ValueNotCalculable, domain.NotCalculable().Status)
	assert.Equal(t, domain.ValueNotApplicable, domain.NotApplicable().Status)
}
