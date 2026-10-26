package domain

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pruebas unitarias obligatorias de Compare (T063, RN-16 a RN-20, SC-012 a SC-014) con la tabla de
// research.md sección 12. Las horas y los porcentajes se expresan en centésimas: un factor de 600 es
// 6 h por Story Point y un porcentaje de 2000 es 20,00 %.
func points(n int) *int { return &n }

// Una historia sin estimar deja la estimada y ambas desviaciones como "no calculable", nunca 0, y
// conserva las horas reales (006/US5, RN-17, SC-012).
func TestCompare_Unestimated(t *testing.T) {
	got := Compare(nil, 600, 1250)

	assert.Nil(t, got.StoryPoints)
	assert.Nil(t, got.EstimatedHundredths)
	assert.Equal(t, 1250, got.ActualHundredths)
	assert.Nil(t, got.DeviationHundredths)
	assert.Nil(t, got.DeviationPercentHundredths)
}

// Con 0 Story Points la estimada es 0, la diferencia absoluta es igual a la real y la porcentual es
// "no calculable" (006/US5, RN-18, SC-013). Las variantes cubren horas reales y ninguna hora.
func TestCompare_ZeroPoints(t *testing.T) {
	t.Run("con horas reales", func(t *testing.T) {
		got := Compare(points(0), 600, 800)

		require.NotNil(t, got.StoryPoints)
		assert.Equal(t, 0, *got.StoryPoints)
		require.NotNil(t, got.EstimatedHundredths)
		assert.Equal(t, 0, *got.EstimatedHundredths)
		assert.Equal(t, 800, got.ActualHundredths)
		require.NotNil(t, got.DeviationHundredths)
		assert.Equal(t, 800, *got.DeviationHundredths)
		assert.Nil(t, got.DeviationPercentHundredths)
	})

	t.Run("sin registros", func(t *testing.T) {
		got := Compare(points(0), 600, 0)

		require.NotNil(t, got.EstimatedHundredths)
		assert.Equal(t, 0, *got.EstimatedHundredths)
		require.NotNil(t, got.DeviationHundredths)
		assert.Equal(t, 0, *got.DeviationHundredths)
		assert.Nil(t, got.DeviationPercentHundredths)
	})
}

// Casos con estimada mayor que 0: estimada = puntos × factor, diferencia = real − estimada y
// porcentaje = diferencia × 10000 / estimada redondeado al final (006/US5, RN-16, RN-19, SC-014).
func TestCompare_Estimated(t *testing.T) {
	cases := []struct {
		name          string
		storyPoints   int
		factor        int
		actual        int
		wantEstimated int
		wantDeviation int
		wantPercent   int
	}{
		{"exceso: 5 puntos, factor 6 y 36 h reales", 5, 600, 3600, 3000, 600, 2000},
		{"ahorro: 5 puntos, factor 6 y 24 h reales", 5, 600, 2400, 3000, -600, -2000},
		{"exacta: 5 puntos, factor 6 y 30 h reales", 5, 600, 3000, 3000, 0, 0},
		{"sin registros: 3 puntos, factor 6", 3, 600, 0, 1800, -1800, -10000},
		{"factor 6,1: 5 puntos y 36 h reales", 5, 610, 3600, 3050, 550, 1803},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Compare(points(c.storyPoints), c.factor, c.actual)

			require.NotNil(t, got.StoryPoints)
			assert.Equal(t, c.storyPoints, *got.StoryPoints)
			require.NotNil(t, got.EstimatedHundredths)
			assert.Equal(t, c.wantEstimated, *got.EstimatedHundredths)
			assert.Equal(t, c.actual, got.ActualHundredths)
			require.NotNil(t, got.DeviationHundredths)
			assert.Equal(t, c.wantDeviation, *got.DeviationHundredths)
			require.NotNil(t, got.DeviationPercentHundredths)
			assert.Equal(t, c.wantPercent, *got.DeviationPercentHundredths)
		})
	}
}

// Cambiar el factor del proyecto recalcula la estimada y las desviaciones sin alterar las horas
// reales (006/US5, RN-20): de factor 6 a factor 8 con 5 puntos y 36 h reales.
func TestCompare_FactorChange(t *testing.T) {
	before := Compare(points(5), 600, 3600)
	after := Compare(points(5), 800, 3600)

	require.NotNil(t, before.EstimatedHundredths)
	require.NotNil(t, after.EstimatedHundredths)
	assert.Equal(t, 3000, *before.EstimatedHundredths)
	assert.Equal(t, 4000, *after.EstimatedHundredths)
	assert.Equal(t, 3600, before.ActualHundredths)
	assert.Equal(t, 3600, after.ActualHundredths)
	require.NotNil(t, after.DeviationHundredths)
	assert.Equal(t, -400, *after.DeviationHundredths)
	require.NotNil(t, after.DeviationPercentHundredths)
	assert.Equal(t, -1000, *after.DeviationPercentHundredths)
}

// Pruebas unitarias obligatorias de RoundPercentHundredths (T063, RN-19, RN-26, SC-022): un único
// redondeo entero de numerator × 10000 / denominator, mitad lejos de cero.
func TestRoundPercentHundredths(t *testing.T) {
	cases := []struct {
		name        string
		numerator   int
		denominator int
		want        int
	}{
		{"un tercio", 1, 3, 3333},
		{"dos tercios", 2, 3, 6667},
		{"menos un tercio", -1, 3, -3333},
		{"menos dos tercios", -2, 3, -6667},
		{"empate positivo: 0,125 % redondea a 0,13 %", 1, 800, 13},
		{"empate negativo: −0,125 % redondea a −0,13 %", -1, 800, -13},
		{"empate positivo en 0,5 centésimas", 1, 20000, 1},
		{"empate negativo en 0,5 centésimas", -1, 20000, -1},
		{"16,5 h sobre 126 h es 13,10 % (008/US1-1)", 1650, 12600, 1310},
		{"cociente exacto", 600, 3000, 2000},
		{"cero", 0, 3000, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, RoundPercentHundredths(c.numerator, c.denominator))
		})
	}
}

// La comparación no usa aritmética de punto flotante (research.md sección 5, SC-022): comparison.go
// no importa el paquete math ni menciona los tipos float32 o float64.
func TestComparison_NoFloatingPoint(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "comparison.go", nil, 0)
	require.NoError(t, err)

	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		require.NoError(t, err)
		assert.NotEqual(t, "math", path)
		assert.NotEqual(t, "math/big", path)
	}

	ast.Inspect(file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			assert.NotContains(t, []string{"float32", "float64"}, id.Name)
		}
		return true
	})
}
