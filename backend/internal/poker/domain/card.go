package domain

import (
	"errors"
	"strconv"

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
func ParseCard(text string) (Card, error) {
	return "", errors.New("not implemented")
}
