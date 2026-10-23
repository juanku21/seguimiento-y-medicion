// Package service implementa la lógica del módulo de Planning Poker: sesiones, presencia, difusión
// y barrido periódico.
package service

import (
	"time"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// SessionService orquesta las sesiones de estimación sobre el repositorio, con el reloj inyectado
// para que las pruebas controlen la gracia, el relevo y la caducidad (research.md secciones 7, 9 y
// 13). Firma mínima de T015: la implementa T023.
type SessionService struct{}

// NewSessionService crea el servicio de sesiones sobre el repositorio y el reloj recibidos. Firma
// mínima de T015: la implementa T023.
func NewSessionService(repo domain.SessionRepository, now func() time.Time) *SessionService {
	return &SessionService{}
}
