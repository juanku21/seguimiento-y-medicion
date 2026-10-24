package domain

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// Card es una carta de votación de Planning Poker; se guarda como texto en la columna
// estimation_votes.card (data-model.md sección 4).
type Card string

// CardUnknown es la carta "?", que se puede jugar pero no es un valor de Story Points (RC-04).
const CardUnknown Card = "?"

// Cards es la lista ordenada de cartas: la escala de Story Points de la feature 003 en orden
// ascendente, seguida de "?". Se arma a partir de backlogdomain.StoryPointsScale para declarar la
// escala una sola vez (RC-04).
var Cards = buildCards()

// buildCards convierte cada valor de la escala de Story Points en su carta y agrega "?" al final.
func buildCards() []Card {
	cards := make([]Card, 0, len(backlogdomain.StoryPointsScale)+1)
	for _, points := range backlogdomain.StoryPointsScale {
		cards = append(cards, Card(strconv.Itoa(points)))
	}
	return append(cards, CardUnknown)
}

// ParseCard convierte el texto recibido en una carta admitida o devuelve un error que enumera las
// cartas en orden (FR-020, FR-025, RN-06).
// La comparación es exacta contra Cards, sin recortar espacios ni interpretar números, así que el
// texto vacío, " 5", "1.5" o "??" se rechazan.
func ParseCard(text string) (Card, error) {
	card := Card(text)
	if !slices.Contains(Cards, card) {
		return "", fmt.Errorf("la carta %q no es válida; cartas admitidas: %s", text, AdmittedCardsList())
	}
	return card, nil
}

// ValidateAgreedValue verifica que el valor acordado pertenezca a la escala numérica de Story Points
// y, si no, devuelve un error que enumera la escala (FR-049, FR-050, RN-18).
// La pertenencia se delega en backlogdomain.IsValidStoryPoints, así que el valor no depende de los
// votos de la ronda: cualquier valor de la escala es válido aunque nadie lo haya votado.
func ValidateAgreedValue(value int) error {
	if !backlogdomain.IsValidStoryPoints(value) {
		return fmt.Errorf("el valor acordado %d no es válido; debe ser uno de %s", value, agreedScaleList())
	}
	return nil
}

// agreedScaleList arma la lista legible de la escala numérica de Story Points en orden, separada por
// comas y con "o" antes del último valor, por ejemplo "0, 1, 2, 3, 5, 8, 13, 20, 40 o 100".
func agreedScaleList() string {
	scale := backlogdomain.StoryPointsScale
	texts := make([]string, len(scale))
	for i, points := range scale {
		texts[i] = strconv.Itoa(points)
	}
	last := len(texts) - 1
	return strings.Join(texts[:last], ", ") + " o " + texts[last]
}

// AdmittedCardsList arma la lista legible de cartas en orden, separadas por comas y con "y" antes de
// la última, por ejemplo "0, 1, 2, 3, 5, 8, 13, 20, 40, 100 y ?". Se exporta para que delivery arme
// su mensaje de validación sin repetir la escala.
func AdmittedCardsList() string {
	texts := make([]string, len(Cards))
	for i, card := range Cards {
		texts[i] = string(card)
	}
	last := len(texts) - 1
	return strings.Join(texts[:last], ", ") + " y " + texts[last]
}
