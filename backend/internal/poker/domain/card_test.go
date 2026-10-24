package domain

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pruebas unitarias obligatorias de ParseCard (T033, RN-06, SC-010): solo las once cartas de la
// escala se aceptan, tal como llegan, y cualquier otro texto se rechaza con un error que enumera las
// cartas admitidas en orden. Los nombres llevan el prefijo "Dominio", convención de las pruebas de
// dominio del proyecto, para no chocar con las pruebas de integración de US2.

// cardsEnumeration es la enumeración ordenada de cartas que el contrato muestra al rechazar una carta
// (contracts/openapi.yaml, ejemplo cartaInvalida).
const cardsEnumeration = "0, 1, 2, 3, 5, 8, 13, 20, 40, 100 y ?"

// requireCardAccepted verifica que el texto se acepte sin error y se convierta en la carta esperada.
func requireCardAccepted(t *testing.T, text string, want Card) {
	t.Helper()
	card, err := ParseCard(text)
	require.NoError(t, err, "la carta %q debía aceptarse", text)
	assert.Equal(t, want, card)
}

// Escenario: Votar en secreto - Caso límite: extremos de la escala (005/US2)
// El 0 y el 100 se aceptan por ser los extremos válidos de la escala; además se verifica que toda
// la escala intermedia de RN-06 se acepte, para que ningún valor admitido quede rechazado.
func TestUS2_DominioExtremosDeLaEscala(t *testing.T) {
	// Los dos extremos de la escala se aceptan.
	t.Run("extremo inferior 0", func(t *testing.T) {
		requireCardAccepted(t, "0", Card("0"))
	})
	t.Run("extremo superior 100", func(t *testing.T) {
		requireCardAccepted(t, "100", Card("100"))
	})

	// Cada una de las once cartas admitidas, incluida "?", se acepta y conserva su texto.
	t.Run("las once cartas admitidas", func(t *testing.T) {
		admitted := []string{"0", "1", "2", "3", "5", "8", "13", "20", "40", "100", "?"}
		for _, text := range admitted {
			requireCardAccepted(t, text, Card(text))
		}
	})
}

// Escenario: Votar en secreto - Caso alternativo: carta "?" (005/US2)
// La carta "?" se acepta como un voto emitido igual que cualquier otra y corresponde a CardUnknown.
func TestUS2_DominioCartaDesconocida(t *testing.T) {
	requireCardAccepted(t, "?", CardUnknown)
}

// Escenario: Votar en secreto - Caso de error: valor fuera de la escala (005/US2)
// Los números intermedios, fuera de rango, negativos y decimales, el texto con espacios, un "?"
// repetido y el texto vacío se rechazan; el error enumera las cartas admitidas en orden (FR-025,
// SC-010). Cada valor rechazado es un subtest.
func TestUS2_DominioValorFueraDeEscala(t *testing.T) {
	rejected := []struct {
		name string
		text string
	}{
		{"intermedio 4", "4"},
		{"intermedio 6", "6"},
		{"intermedio 7", "7"},
		{"intermedio 50", "50"},
		{"fuera de rango 101", "101"},
		{"negativo -1", "-1"},
		{"decimal 1.5", "1.5"},
		{"espacio inicial", " 5"},
		{"signo repetido", "??"},
		{"texto vacio", ""},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			card, err := ParseCard(tc.text)

			require.Error(t, err, "la carta %q debía rechazarse", tc.text)
			assert.Contains(t, err.Error(), cardsEnumeration)
			assert.Equal(t, Card(""), card)
		})
	}
}

// Pruebas unitarias obligatorias de ValidateAgreedValue (T061, RN-18): el valor acordado es un valor
// numérico de la escala, elegido por el facilitador y sin relación con los votos, así que se acepta
// cualquiera de los diez valores y se rechaza todo lo demás con un error que enumera la escala.

// agreedScaleEnumeration es la enumeración de la escala numérica que el contrato muestra al rechazar
// un valor acordado (contracts/openapi.yaml, mensaje de storyPoints).
const agreedScaleEnumeration = "0, 1, 2, 3, 5, 8, 13, 20, 40 o 100"

// Escenario: Registrar la estimación acordada - Caso alternativo: valor que nadie votó (005/US5)
// La validación no depende de los votos: cada uno de los diez valores de la escala se acepta como
// valor acordado, incluido el 13 que nadie votó y el 40 de una ronda con todos "?". Cada valor es un
// subtest.
func TestUS5_DominioValorAcordadoDeLaEscala(t *testing.T) {
	for _, value := range []int{0, 1, 2, 3, 5, 8, 13, 20, 40, 100} {
		t.Run(strconv.Itoa(value), func(t *testing.T) {
			assert.NoError(t, ValidateAgreedValue(value), "el valor %d debía aceptarse", value)
		})
	}
}

// Escenario: Registrar la estimación acordada - Caso de error: valor fuera de la escala (005/US5)
// Los intermedios 7 y 4, el negativo -1 y el 101 fuera de rango se rechazan, y el error enumera los
// valores admitidos de la escala (FR-050). Cada valor rechazado es un subtest.
func TestUS5_DominioValorAcordadoFueraDeEscala(t *testing.T) {
	for _, value := range []int{7, 4, -1, 101} {
		t.Run(strconv.Itoa(value), func(t *testing.T) {
			err := ValidateAgreedValue(value)

			require.Error(t, err, "el valor %d debía rechazarse", value)
			assert.Contains(t, err.Error(), agreedScaleEnumeration)
		})
	}
}
