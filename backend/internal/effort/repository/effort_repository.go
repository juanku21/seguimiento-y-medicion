// Package repository implementa con GORM la persistencia del módulo de registro de esfuerzo
// (Principio III).
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// EffortRepository implementa domain.EffortRepository con GORM sobre la tabla effort_entries. Por
// ahora están implementados Create y ListForStory; el resto de las operaciones corresponde a tareas
// posteriores.
type EffortRepository struct {
	db *gorm.DB
}

// NewEffortRepository crea el repositorio de esfuerzo sobre la conexión recibida.
func NewEffortRepository(db *gorm.DB) *EffortRepository {
	return &EffortRepository{db: db}
}

// FindAccessible resuelve el acceso a un registro para RequireEffortEntryAccess. Firma mínima de
// T015: la implementación corresponde a T050.
func (r *EffortRepository) FindAccessible(ctx context.Context, entryID, userID uuid.UUID) (*domain.EntryAccess, error) {
	return nil, errors.New("not implemented")
}

// Create registra el alta de un registro de esfuerzo en una transacción, con el orden de bloqueos de
// research.md sección 10: (1) bloquea con FOR SHARE el proyecto de la historia y rechaza el proyecto
// Finalizado con ErrEffortReadOnly antes de mirar el plazo de gracia (FR-008); (2) valida la fecha
// contra hoy en la zona recibida y el inicio del proyecto (RN-05); (3) lee sin bloqueo los compromisos
// de la historia y resuelve el sprint con ResolveAnchor (FR-014, FR-016); (4) bloquea con FOR NO KEY
// UPDATE la fila del autor en users, suma sus horas de la fecha en todos los proyectos y aplica el
// tope diario (FR-005, RN-04); (5) inserta el registro, y (6) marca la historia con ever_had_effort
// sin tocar updated_at (RC-05). Devuelve el registro con su autor (solo id y nombre completo), su
// historia y su sprint, que es nil si quedó sin sprint.
func (r *EffortRepository) Create(ctx context.Context, storyID, authorID uuid.UUID, input domain.EntryInput, now time.Time, location *time.Location, grace time.Duration) (*domain.EntryView, error) {
	var view *domain.EntryView
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		story, err := lockStoryProject(tx, storyID)
		if err != nil {
			return err
		}
		if story.ProjectStatus == projectsdomain.ProjectStatusFinished {
			return domain.ErrEffortReadOnly
		}
		if err := domain.ValidateEntryDate(input.EntryDate, story.ProjectStartDate, calendarToday(now, location)); err != nil {
			return err
		}

		commitments, err := storyCommitments(tx, storyID)
		if err != nil {
			return err
		}
		sprintID, err := domain.ResolveAnchor(commitments.open(), commitments.lastClosed(), now, grace)
		if err != nil {
			return err
		}

		author, err := lockAuthor(tx, authorID)
		if err != nil {
			return err
		}
		logged, err := loggedHundredths(tx, authorID, input.EntryDate)
		if err != nil {
			return err
		}
		if err := domain.CheckDailyCap(logged, input.HoursHundredths); err != nil {
			return err
		}

		entry := domain.EffortEntry{
			ID:              uuid.New(),
			StoryID:         storyID,
			UserID:          authorID,
			SprintID:        sprintID,
			EntryDate:       input.EntryDate,
			Activity:        input.Activity,
			HoursHundredths: input.HoursHundredths,
		}
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("insertar el registro de esfuerzo: %w", err)
		}

		err = tx.
			Model(&backlogdomain.UserStory{}).
			Where("id = ? AND ever_had_effort = ?", storyID, false).
			UpdateColumn("ever_had_effort", true).Error
		if err != nil {
			return fmt.Errorf("marcar la historia con esfuerzo: %w", err)
		}

		view = &domain.EntryView{
			Entry:  entry,
			Author: *author,
			Story:  domain.StoryRef{ID: storyID, Title: story.Title},
			Sprint: commitments.sprint(sprintID),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// lockedStoryRow es la fila que lee lockStoryProject: el título de la historia y, por el JOIN a
// projects, el estado y la fecha de inicio de su proyecto con los alias project_status y
// project_start_date.
type lockedStoryRow struct {
	Title            string
	ProjectStatus    projectsdomain.ProjectStatus
	ProjectStartDate time.Time
}

// lockStoryProject lee, dentro de la transacción recibida, la historia unida a su proyecto con un
// bloqueo FOR SHARE solo sobre la fila de projects, para que el proyecto no pase a Finalizado hasta
// que termine la transacción (patrón lockWritableStoryProject de la 003). Si no hay fila, la historia
// no existe y devuelve ErrStoryNotFound.
func lockStoryProject(tx *gorm.DB, storyID uuid.UUID) (*lockedStoryRow, error) {
	var row lockedStoryRow
	err := tx.
		Table("user_stories").
		Select(`user_stories.title, projects.status AS project_status,
			projects.start_date AS project_start_date`).
		Joins("JOIN projects ON projects.id = user_stories.project_id").
		Where("user_stories.id = ?", storyID).
		Clauses(clause.Locking{Strength: "SHARE", Table: clause.Table{Name: "projects"}}).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, backlogdomain.ErrStoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("bloquear el proyecto de la historia: %w", err)
	}
	return &row, nil
}

// calendarToday devuelve la fecha de calendario de now en la zona recibida, como medianoche UTC, que
// es como se representan las columnas date (research.md sección 7).
func calendarToday(now time.Time, location *time.Location) time.Time {
	year, month, day := now.In(location).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// commitmentRow es un compromiso de la historia que lee storyCommitments: el sprint con su nombre,
// estado y cierre, y el resultado congelado, que es nil mientras el compromiso está abierto.
type commitmentRow struct {
	SprintID     uuid.UUID
	SprintName   string
	SprintStatus sprintsdomain.SprintStatus
	ClosedAt     *time.Time
	Outcome      *sprintsdomain.StoryOutcome
}

// commitments son los compromisos de una historia ordenados con los abiertos primero y luego por
// cierre más reciente, tal como los devuelve storyCommitments.
type commitments []commitmentRow

// storyCommitments lee sin bloqueo, en una sola sentencia y por lo tanto con una sola instantánea,
// todos los compromisos de la historia unidos a su sprint (research.md sección 10, paso 3). El orden
// por sprints.closed_at DESC NULLS FIRST deja primero el compromiso abierto, cuyo sprint no está
// cerrado, y después los congelados del cierre más reciente al más antiguo.
func storyCommitments(tx *gorm.DB, storyID uuid.UUID) (commitments, error) {
	rows := commitments{}
	err := tx.
		Table("sprint_stories").
		Select(`sprint_stories.sprint_id, sprints.name AS sprint_name, sprints.status AS sprint_status,
			sprints.closed_at, sprint_stories.outcome`).
		Joins("JOIN sprints ON sprints.id = sprint_stories.sprint_id").
		Where("sprint_stories.story_id = ?", storyID).
		Order("sprints.closed_at DESC NULLS FIRST, sprint_stories.id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("leer los compromisos de la historia: %w", err)
	}
	return rows, nil
}

// open devuelve el compromiso abierto (outcome NULL) de la historia, o nil si no lo hay; la 004
// garantiza que hay como máximo uno.
func (c commitments) open() *domain.OpenCommitment {
	for _, row := range c {
		if row.Outcome == nil {
			return &domain.OpenCommitment{SprintID: row.SprintID}
		}
	}
	return nil
}

// lastClosed devuelve el compromiso congelado del sprint de cierre más reciente con su resultado, o
// nil si la historia nunca quedó en un sprint cerrado. Por el orden de storyCommitments es el primer
// compromiso con outcome; uno congelado sin closed_at no debería existir y se ignora.
func (c commitments) lastClosed() *domain.ClosedCommitment {
	for _, row := range c {
		if row.Outcome != nil && row.ClosedAt != nil {
			return &domain.ClosedCommitment{SprintID: row.SprintID, ClosedAt: *row.ClosedAt, Outcome: *row.Outcome}
		}
	}
	return nil
}

// sprint arma el sprint asociado de la respuesta a partir del compromiso leído del sprint resuelto,
// sin volver a consultar la base; devuelve nil si el registro quedó sin sprint.
func (c commitments) sprint(sprintID *uuid.UUID) *domain.AnchoredSprint {
	if sprintID == nil {
		return nil
	}
	for _, row := range c {
		if row.SprintID == *sprintID {
			return &domain.AnchoredSprint{ID: row.SprintID, Name: row.SprintName, Status: row.SprintStatus, ClosedAt: row.ClosedAt}
		}
	}
	return nil
}

// lockAuthor bloquea con FOR NO KEY UPDATE la fila del autor en users y lee solo su id y su nombre
// completo, nunca password_hash. El bloqueo serializa las escrituras de esfuerzo de esa persona para
// que el tope diario sea una invariante ante pedidos simultáneos, sin bloquear las inserciones que
// referencian al usuario con FOR KEY SHARE (research.md sección 9).
func lockAuthor(tx *gorm.DB, authorID uuid.UUID) (*domain.UserRef, error) {
	var author domain.UserRef
	err := tx.
		Table("users").
		Select("id, full_name").
		Where("id = ?", authorID).
		Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).
		Take(&author).Error
	if err != nil {
		return nil, fmt.Errorf("bloquear al autor del registro: %w", err)
	}
	return &author, nil
}

// loggedHundredths suma las centésimas de hora que la persona ya registró en la fecha, en todos los
// proyectos, incluidos los Finalizados y aquellos de los que fue quitada (RC-09).
func loggedHundredths(tx *gorm.DB, userID uuid.UUID, date time.Time) (int, error) {
	var total int
	err := tx.
		Model(&domain.EffortEntry{}).
		Select("COALESCE(SUM(hours_hundredths), 0)").
		Where("user_id = ? AND entry_date = ?", userID, date).
		Scan(&total).Error
	if err != nil {
		return 0, fmt.Errorf("sumar las horas del día: %w", err)
	}
	return total, nil
}

// Update modifica un registro propio en una transacción. Firma mínima de T015: la implementación
// corresponde a T050.
func (r *EffortRepository) Update(ctx context.Context, entryID, actorID uuid.UUID, patch domain.EntryPatch, now time.Time, location *time.Location, grace time.Duration) (*domain.EntryView, error) {
	return nil, errors.New("not implemented")
}

// Delete elimina un registro propio en una transacción. Firma mínima de T015: la implementación
// corresponde a T050.
func (r *EffortRepository) Delete(ctx context.Context, entryID, actorID uuid.UUID, now time.Time, grace time.Duration) error {
	return errors.New("not implemented")
}

// entryRow es la fila plana que leen las consultas de listado: el registro, el nombre completo de su
// autor y el título de su historia, y los datos del sprint asociado, que son NULL cuando el registro
// no tiene sprint por el LEFT JOIN.
type entryRow struct {
	domain.EffortEntry
	AuthorFullName string
	StoryTitle     string
	SprintName     *string
	SprintStatus   *sprintsdomain.SprintStatus
	SprintClosedAt *time.Time
}

// view arma el EntryView de la fila; el sprint queda nil si el registro no tiene sprint.
func (row entryRow) view() domain.EntryView {
	view := domain.EntryView{
		Entry:  row.EffortEntry,
		Author: domain.UserRef{ID: row.UserID, FullName: row.AuthorFullName},
		Story:  domain.StoryRef{ID: row.StoryID, Title: row.StoryTitle},
	}
	if row.SprintID != nil && row.SprintName != nil && row.SprintStatus != nil {
		view.Sprint = &domain.AnchoredSprint{
			ID:       *row.SprintID,
			Name:     *row.SprintName,
			Status:   *row.SprintStatus,
			ClosedAt: row.SprintClosedAt,
		}
	}
	return view
}

// ListForStory devuelve los registros de una historia en una sola consulta (research.md sección
// 11): effort_entries unido a users (solo id y nombre completo, nunca password_hash), a user_stories
// para el título y con LEFT JOIN a sprints para el nombre, el estado y el cierre del sprint asociado.
// El orden es el de FR-035: fecha descendente, creación ascendente e id ascendente como desempate.
// Una historia sin registros devuelve una lista vacía, nunca nil.
func (r *EffortRepository) ListForStory(ctx context.Context, storyID uuid.UUID) ([]domain.EntryView, error) {
	rows := []entryRow{}
	err := r.db.WithContext(ctx).
		Table("effort_entries").
		Select(`effort_entries.id, effort_entries.story_id, effort_entries.user_id,
			effort_entries.sprint_id, effort_entries.entry_date, effort_entries.activity,
			effort_entries.hours_hundredths, effort_entries.created_at, effort_entries.updated_at,
			users.full_name AS author_full_name, user_stories.title AS story_title,
			sprints.name AS sprint_name, sprints.status AS sprint_status,
			sprints.closed_at AS sprint_closed_at`).
		Joins("JOIN users ON users.id = effort_entries.user_id").
		Joins("JOIN user_stories ON user_stories.id = effort_entries.story_id").
		Joins("LEFT JOIN sprints ON sprints.id = effort_entries.sprint_id").
		Where("effort_entries.story_id = ?", storyID).
		Order("effort_entries.entry_date DESC, effort_entries.created_at ASC, effort_entries.id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("listar el esfuerzo de la historia: %w", err)
	}

	views := make([]domain.EntryView, 0, len(rows))
	for _, row := range rows {
		views = append(views, row.view())
	}
	return views, nil
}

// ListForProject devuelve los registros de un proyecto según el filtro. Firma mínima de T015: la
// implementación corresponde a T059.
func (r *EffortRepository) ListForProject(ctx context.Context, projectID uuid.UUID, filter domain.EntryFilter) ([]domain.EntryView, error) {
	return nil, errors.New("not implemented")
}

// ValidateFilter verifica el filtro contra el proyecto. Firma mínima de T015: la implementación
// corresponde a T059.
func (r *EffortRepository) ValidateFilter(ctx context.Context, projectID uuid.UUID, filter domain.EntryFilter) error {
	return errors.New("not implemented")
}

// HasProjectEffort informa si el proyecto tiene algún registro de esfuerzo. Firma mínima de T015:
// la implementación corresponde a T059.
func (r *EffortRepository) HasProjectEffort(ctx context.Context, projectID uuid.UUID) (bool, error) {
	return false, errors.New("not implemented")
}

// ComparisonRows devuelve el factor vigente del proyecto y las filas de la comparación. Firma mínima
// de T015: la implementación corresponde a T067.
func (r *EffortRepository) ComparisonRows(ctx context.Context, projectID uuid.UUID, storyID *uuid.UUID) (int, []domain.ComparisonRow, error) {
	return 0, nil, errors.New("not implemented")
}
