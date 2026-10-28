package domain_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	effortdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
)

// Escenario: denominador cero produce not_calculable (008/US1, RN-19)
func TestUS1_PorcentajeDenominadorCero(t *testing.T) {
	for _, n := range []int{0, 5, -5} {
		t.Run(fmt.Sprintf("numerador_%d", n), func(t *testing.T) {
			assert.Equal(t, domain.NotCalculable(), domain.Percent(n, 0))
		})
	}
}

// Escenario: porcentaje en centésimas con un único redondeo (008/US1, RN-17, RN-20)
func TestUS1_PorcentajeRedondeo(t *testing.T) {
	cases := []struct {
		numerator, denominator, want int
	}{
		{1650, 12600, 1310},
		{-1600, 12600, -1270},
		{2, 4, 5000},
		{1, 3, 3333},
		{2, 3, 6667},
		{7, 11, 6364},
		{-6550, 37800, -1733},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%d_sobre_%d", c.numerator, c.denominator), func(t *testing.T) {
			assert.Equal(t, domain.Available(c.want), domain.Percent(c.numerator, c.denominator))
		})
	}
}

// Escenario: empates en ,005 redondean lejos de cero en ambos signos (008/US1, RN-20)
func TestUS1_PorcentajeEmpateLejosDeCero(t *testing.T) {
	cases := []struct {
		numerator, denominator, want int
	}{
		{1, 16, 625},   // 6,25 exacto, sin empate: referencia
		{1, 32, 313},   // 3,125 -> 3,13
		{-1, 32, -313}, // simétrico negativo
		{3, 32, 938},   // 9,375 -> 9,38
		{-3, 32, -938}, // simétrico negativo
		{1, -32, -313}, // denominador negativo
		{-1, -32, 313}, // ambos negativos
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%d_sobre_%d", c.numerator, c.denominator), func(t *testing.T) {
			got := domain.Percent(c.numerator, c.denominator)
			assert.Equal(t, domain.Available(c.want), got)
			// Debe coincidir con la función de referencia del módulo de esfuerzo.
			assert.Equal(t, effortdomain.RoundPercentHundredths(c.numerator, c.denominator), got.Value)
		})
	}
}
