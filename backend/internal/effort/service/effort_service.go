// Package service resuelve los casos de uso del registro de esfuerzo sobre el repositorio del
// módulo, con el reloj, la zona horaria y el plazo de gracia inyectados (Principio III).
package service

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
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

// StoryEffort devuelve el esfuerzo de una historia: los registros en el orden de FR-035, el total y
// el desglose por integrante (FR-030 a FR-035). La lista se lee una sola vez y el total y el desglose
// se agregan en Go sobre ella, para que no puedan ver otra instantánea que la lista (research.md
// sección 11). El instante se toma del reloj una sola vez para que todos los registros evalúen la
// condición de definitivo sobre el mismo momento.
func (s *EffortService) StoryEffort(ctx context.Context, storyID uuid.UUID) (*domain.StoryEffort, error) {
	entries, err := s.repo.ListForStory(ctx, storyID)
	if err != nil {
		return nil, fmt.Errorf("consultar el esfuerzo de la historia: %w", err)
	}

	now := s.now()
	for i := range entries {
		entries[i].Definitive = isDefinitive(entries[i].Sprint, now, s.grace)
	}

	total, byMember := aggregateByMember(entries)
	return &domain.StoryEffort{TotalHundredths: total, ByMember: byMember, Entries: entries}, nil
}

// isDefinitive informa si un registro ya no se modifica ni se elimina: su sprint asociado está
// Cerrado y el plazo de gracia ya no está vigente en now (FR-020, RN-09). Un registro sin sprint, o
// con un sprint no cerrado, nunca es definitivo; un sprint cerrado sin closed_at no debería existir y
// se trata como no definitivo.
func isDefinitive(sprint *domain.AnchoredSprint, now time.Time, grace time.Duration) bool {
	if sprint == nil || sprint.Status != sprintsdomain.SprintStatusClosed || sprint.ClosedAt == nil {
		return false
	}
	return !domain.GraceOpen(*sprint.ClosedAt, now, grace)
}

// aggregateByMember suma las centésimas de hora de los registros en un total y en una línea por
// autor, ordenadas por nombre completo y luego por identificador (research.md sección 11). Sin
// registros devuelve un total de cero y un desglose vacío, nunca nil.
func aggregateByMember(entries []domain.EntryView) (int, []domain.MemberEffort) {
	total := 0
	byMember := []domain.MemberEffort{}
	positions := map[uuid.UUID]int{}
	for _, entry := range entries {
		total += entry.Entry.HoursHundredths
		pos, ok := positions[entry.Author.ID]
		if !ok {
			pos = len(byMember)
			positions[entry.Author.ID] = pos
			byMember = append(byMember, domain.MemberEffort{UserID: entry.Author.ID, FullName: entry.Author.FullName})
		}
		byMember[pos].HoursHundredths += entry.Entry.HoursHundredths
	}

	slices.SortFunc(byMember, func(a, b domain.MemberEffort) int {
		return cmp.Or(
			strings.Compare(a.FullName, b.FullName),
			bytes.Compare(a.UserID[:], b.UserID[:]),
		)
	})
	return total, byMember
}
