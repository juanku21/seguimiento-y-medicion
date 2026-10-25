// Package service resuelve los casos de uso del registro de esfuerzo sobre el repositorio del
// módulo, con el reloj, la zona horaria y el plazo de gracia inyectados (Principio III).
package service

import (
	"time"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
)

// EffortService reúne los casos de uso del registro de esfuerzo. Firma mínima de T015: la
// implementación corresponde a T027.
type EffortService struct{}

// NewEffortService compone el servicio con el repositorio, la zona horaria del sistema, el plazo de
// gracia y el reloj. Firma mínima de T015: la implementación corresponde a T027.
func NewEffortService(repo domain.EffortRepository, location *time.Location, grace time.Duration, now func() time.Time) *EffortService {
	return nil
}
