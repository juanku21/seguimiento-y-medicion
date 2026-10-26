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

// Update corrige un registro propio con el parche ya validado y devuelve el registro actualizado
// (FR-020 a FR-024). El actor es siempre actorID, el titular de la sesión, y el instante se toma del
// reloj una sola vez para que la fecha de hoy y la vigencia del plazo de gracia se evalúen sobre el
// mismo momento. Las reglas que dependen de datos (solo lectura del proyecto, autoría, registro
// definitivo, fecha y tope diario) las aplica el repositorio dentro de su transacción; sus rechazos
// se envuelven con %w para que delivery los reconozca con errors.Is y errors.As.
func (s *EffortService) Update(ctx context.Context, entryID, actorID uuid.UUID, patch domain.EntryPatch) (*domain.EntryView, error) {
	view, err := s.repo.Update(ctx, entryID, actorID, patch, s.now(), s.location, s.grace)
	if err != nil {
		return nil, fmt.Errorf("corregir esfuerzo: %w", err)
	}

	return view, nil
}

// Delete elimina un registro propio (FR-020 a FR-022, FR-027) con el actor de la sesión y el instante
// del reloj. El repositorio aplica la solo lectura del proyecto, la autoría y la condición de
// definitivo dentro de su transacción; sus rechazos se envuelven con %w.
func (s *EffortService) Delete(ctx context.Context, entryID, actorID uuid.UUID) error {
	if err := s.repo.Delete(ctx, entryID, actorID, s.now(), s.grace); err != nil {
		return fmt.Errorf("eliminar esfuerzo: %w", err)
	}

	return nil
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

	s.markDefinitive(entries)
	total, byMember := aggregateByMember(entries)
	return &domain.StoryEffort{TotalHundredths: total, ByMember: byMember, Entries: entries}, nil
}

// EmptyReason explica por qué la consulta de esfuerzo del proyecto vino vacía, para que la pantalla
// distinga el proyecto sin esfuerzo de los filtros sin coincidencias (US4-7, US4-8). Viaja como texto
// en el campo emptyReason del contrato; su ausencia (nil) significa que hay resultados.
type EmptyReason string

// Motivos posibles de una consulta vacía: el proyecto no tiene ningún registro, aunque se haya
// filtrado, o los tiene pero ninguno cumple los filtros.
const (
	EmptyReasonNoEffort  EmptyReason = "no_effort"
	EmptyReasonNoMatches EmptyReason = "no_matches"
)

// ProjectEffortView es el resultado de la consulta de esfuerzo del proyecto: el total en centésimas
// de hora, los registros en el orden de FR-045, nunca nil, y el motivo del vacío, nil cuando hay
// resultados.
type ProjectEffortView struct {
	TotalHundredths int
	Entries         []domain.EntryView
	EmptyReason     *EmptyReason
}

// ProjectEffort devuelve el esfuerzo del proyecto según los filtros combinables, con el total de lo
// filtrado (FR-036 a FR-045). Primero verifica que los filtros correspondan al proyecto (FR-044): el
// rechazo se envuelve con %w para que delivery lo reconozca con errors.As. La lista se lee una sola
// vez y el total se agrega en Go sobre ella (research.md sección 11). Solo con la lista vacía se
// consulta la existencia de esfuerzo para elegir el motivo: un proyecto sin registros es no_effort
// aunque haya filtros.
func (s *EffortService) ProjectEffort(ctx context.Context, projectID uuid.UUID, filter domain.EntryFilter) (*ProjectEffortView, error) {
	if err := s.repo.ValidateFilter(ctx, projectID, filter); err != nil {
		return nil, fmt.Errorf("validar el filtro del esfuerzo del proyecto: %w", err)
	}

	entries, err := s.repo.ListForProject(ctx, projectID, filter)
	if err != nil {
		return nil, fmt.Errorf("consultar el esfuerzo del proyecto: %w", err)
	}
	if len(entries) > 0 {
		s.markDefinitive(entries)
		return &ProjectEffortView{TotalHundredths: sumHundredths(entries), Entries: entries}, nil
	}

	// Resultado vacío: se consulta si el proyecto tiene algún registro para elegir el motivo.
	hasEffort, err := s.repo.HasProjectEffort(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("verificar si el proyecto tiene esfuerzo: %w", err)
	}
	reason := EmptyReasonNoEffort
	if hasEffort {
		reason = EmptyReasonNoMatches
	}
	return &ProjectEffortView{Entries: []domain.EntryView{}, EmptyReason: &reason}, nil
}

// markDefinitive completa la condición de definitivo de cada registro. El instante se toma del reloj
// una sola vez para que todos los registros la evalúen sobre el mismo momento.
func (s *EffortService) markDefinitive(entries []domain.EntryView) {
	now := s.now()
	for i := range entries {
		entries[i].Definitive = isDefinitive(entries[i].Sprint, now, s.grace)
	}
}

// sumHundredths suma las centésimas de hora de los registros; sin registros devuelve cero.
func sumHundredths(entries []domain.EntryView) int {
	total := 0
	for _, entry := range entries {
		total += entry.Entry.HoursHundredths
	}
	return total
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
