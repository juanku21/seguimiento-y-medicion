// Package service resuelve los casos de uso del registro de esfuerzo sobre el repositorio del
// módulo, con el reloj, la zona horaria y el plazo de gracia inyectados (Principio III).
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
)

// EffortService reúne los casos de uso del registro de esfuerzo. Depende del repositorio, de la zona
// horaria única del sistema, en la que se calcula el día de hoy, del plazo de gracia para registrar
// después del cierre de un sprint y de un reloj inyectado que permite fijar los instantes en las
// pruebas.
type EffortService struct {
	repo     domain.EffortRepository
	location *time.Location
	grace    time.Duration
	now      func() time.Time
}

// NewEffortService compone el servicio con el repositorio, la zona horaria del sistema, el plazo de
// gracia y el reloj. La firma es la definitiva de la feature: las historias siguientes usan las
// mismas dependencias sin cambiarla.
func NewEffortService(repo domain.EffortRepository, location *time.Location, grace time.Duration, now func() time.Time) *EffortService {
	return &EffortService{repo: repo, location: location, grace: grace, now: now}
}

// Create registra el esfuerzo del titular de la sesión sobre la historia del acceso y devuelve el
// registro creado (FR-001 a FR-016). El autor es siempre actorID, nunca un dato del cuerpo (FR-002),
// y el instante se toma del reloj una sola vez para que la fecha de hoy y la vigencia del plazo de
// gracia se evalúen sobre el mismo momento. Las reglas que dependen de datos (solo lectura del
// proyecto, fecha, anclaje al sprint y tope diario) las aplica el repositorio dentro de su
// transacción; sus rechazos se envuelven con %w para que delivery los reconozca con errors.Is y
// errors.As.
func (s *EffortService) Create(ctx context.Context, access *backlogdomain.StoryAccess, actorID uuid.UUID, input domain.EntryInput) (*domain.EntryView, error) {
	view, err := s.repo.Create(ctx, access.StoryID, actorID, input, s.now(), s.location, s.grace)
	if err != nil {
		return nil, fmt.Errorf("registrar esfuerzo: %w", err)
	}

	return view, nil
}
